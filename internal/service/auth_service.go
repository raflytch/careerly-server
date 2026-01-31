package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/raflytch/careerly-server/internal/config"
	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/jwt"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	ErrFailedToExchangeToken = errors.New("failed to exchange token")
	ErrFailedToGetUserInfo   = errors.New("failed to get user info")
	ErrUserNotActive         = errors.New("user account is not active")
)

type authService struct {
	userRepo    domain.UserRepository
	cacheRepo   domain.CacheRepository
	oauthConfig *oauth2.Config
	jwtManager  *jwt.JWTManager
}

func NewAuthService(
	userRepo domain.UserRepository,
	cacheRepo domain.CacheRepository,
	cfg config.GoogleConfig,
	jwtManager *jwt.JWTManager,
) domain.AuthService {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &authService{
		userRepo:    userRepo,
		cacheRepo:   cacheRepo,
		oauthConfig: oauthConfig,
		jwtManager:  jwtManager,
	}
}

func (s *authService) GetGoogleLoginURL(state string) string {
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *authService) HandleGoogleCallback(ctx context.Context, code string) (*domain.AuthResponse, error) {
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, ErrFailedToExchangeToken
	}

	client := s.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, ErrFailedToGetUserInfo
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrFailedToGetUserInfo
	}

	var googleUser domain.GoogleUserInfo
	if err := json.Unmarshal(body, &googleUser); err != nil {
		return nil, ErrFailedToGetUserInfo
	}

	user, err := s.userRepo.FindByGoogleID(ctx, googleUser.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			user = &domain.User{
				ID:        uuid.New(),
				GoogleID:  googleUser.ID,
				Email:     googleUser.Email,
				Name:      googleUser.Name,
				AvatarURL: &googleUser.Picture,
				Role:      domain.RoleUser,
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			if err := s.userRepo.Create(ctx, user); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if !user.IsActive {
		return nil, ErrUserNotActive
	}

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, err
	}

	jwtToken, err := s.jwtManager.Generate(user.ID, user.Email, string(user.Role))
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, user.ID.String())
	_ = s.cacheRepo.Set(ctx, cacheKey, user, userCacheDuration)

	return &domain.AuthResponse{
		Token: jwtToken,
		User:  *user,
	}, nil
}

func (s *authService) ValidateToken(ctx context.Context, tokenString string) (*domain.User, error) {
	claims, err := s.jwtManager.Validate(tokenString)
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, claims.UserID.String())
	cached, err := s.cacheRepo.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		var user domain.User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			if user.IsActive {
				return &user, nil
			}
			return nil, ErrUserNotActive
		}
	}

	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserNotActive
	}

	_ = s.cacheRepo.Set(ctx, cacheKey, user, userCacheDuration)

	return user, nil
}
