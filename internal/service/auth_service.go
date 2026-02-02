package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/raflytch/careerly-server/internal/config"
	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/jwt"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	userCachePrefix   = "user:"
	userCacheDuration = 15 * time.Minute
	otpCachePrefix    = "otp:restore:"
	otpCacheDuration  = 15 * time.Minute
	otpLength         = 6
)

var (
	ErrFailedToExchangeToken = errors.New("failed to exchange token")
	ErrFailedToGetUserInfo   = errors.New("failed to get user info")
	ErrUserNotActive         = errors.New("user account is not active")
)

type authService struct {
	userRepo     domain.UserRepository
	cacheRepo    domain.CacheRepository
	emailService domain.EmailService
	oauthConfig  *oauth2.Config
	jwtManager   *jwt.JWTManager
}

func NewAuthService(
	userRepo domain.UserRepository,
	cacheRepo domain.CacheRepository,
	emailService domain.EmailService,
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
		userRepo:     userRepo,
		cacheRepo:    cacheRepo,
		emailService: emailService,
		oauthConfig:  oauthConfig,
		jwtManager:   jwtManager,
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
			deletedUser, delErr := s.userRepo.FindDeletedByGoogleID(ctx, googleUser.ID)
			if delErr == nil && deletedUser != nil {
				return nil, domain.ErrUserDeleted
			}

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
				if s.isDuplicateKeyError(err) {
					deletedUser, delErr := s.userRepo.FindDeletedByGoogleID(ctx, googleUser.ID)
					if delErr == nil && deletedUser != nil {
						return nil, domain.ErrUserDeleted
					}
				}
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

func (s *authService) RequestRestoreOTP(ctx context.Context, email string) (*domain.OTPResponse, error) {
	deletedUser, err := s.userRepo.FindDeletedByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoDeletedUserFound
		}
		return nil, err
	}

	if deletedUser.DeletedAt == nil {
		return nil, domain.ErrUserAlreadyActive
	}

	otpKey := fmt.Sprintf("%s%s", otpCachePrefix, email)
	existingOTP, err := s.cacheRepo.Get(ctx, otpKey)
	if err == nil && existingOTP != "" {
		return nil, domain.ErrOTPAlreadySent
	}

	otp, err := s.generateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	if err := s.cacheRepo.Set(ctx, otpKey, otp, otpCacheDuration); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	if err := s.emailService.SendOTP(ctx, email, otp); err != nil {
		_ = s.cacheRepo.Delete(ctx, otpKey)
		return nil, fmt.Errorf("failed to send OTP email: %w", err)
	}

	return &domain.OTPResponse{
		Message:   "OTP has been sent to your email address",
		ExpiresIn: int(otpCacheDuration.Seconds()),
	}, nil
}

func (s *authService) VerifyRestoreOTP(ctx context.Context, email, otp string) (*domain.RestoreUserResponse, error) {
	otpKey := fmt.Sprintf("%s%s", otpCachePrefix, email)
	storedOTP, err := s.cacheRepo.Get(ctx, otpKey)
	if err != nil {
		return nil, domain.ErrInvalidOTP
	}

	storedOTP = strings.Trim(storedOTP, "\"")
	if storedOTP != otp {
		return nil, domain.ErrInvalidOTP
	}

	deletedUser, err := s.userRepo.FindDeletedByEmail(ctx, email)
	if err != nil {
		return nil, domain.ErrNoDeletedUserFound
	}

	if err := s.userRepo.Restore(ctx, deletedUser.ID); err != nil {
		return nil, fmt.Errorf("failed to restore user: %w", err)
	}

	_ = s.cacheRepo.Delete(ctx, otpKey)

	restoredUser, err := s.userRepo.FindByID(ctx, deletedUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch restored user: %w", err)
	}

	return &domain.RestoreUserResponse{
		Message: "Your account has been successfully restored. You can now login again.",
		User:    *restoredUser,
	}, nil
}

func (s *authService) ResendRestoreOTP(ctx context.Context, email string) (*domain.OTPResponse, error) {
	deletedUser, err := s.userRepo.FindDeletedByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoDeletedUserFound
		}
		return nil, err
	}

	if deletedUser.DeletedAt == nil {
		return nil, domain.ErrUserAlreadyActive
	}

	otpKey := fmt.Sprintf("%s%s", otpCachePrefix, email)
	_ = s.cacheRepo.Delete(ctx, otpKey)

	otp, err := s.generateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	if err := s.cacheRepo.Set(ctx, otpKey, otp, otpCacheDuration); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	if err := s.emailService.SendOTP(ctx, email, otp); err != nil {
		_ = s.cacheRepo.Delete(ctx, otpKey)
		return nil, fmt.Errorf("failed to send OTP email: %w", err)
	}

	return &domain.OTPResponse{
		Message:   "A new OTP has been sent to your email address",
		ExpiresIn: int(otpCacheDuration.Seconds()),
	}, nil
}

func (s *authService) generateOTP() (string, error) {
	const digits = "0123456789"
	otp := make([]byte, otpLength)
	for i := 0; i < otpLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		otp[i] = digits[n.Int64()]
	}
	return string(otp), nil
}

func (s *authService) isDuplicateKeyError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
