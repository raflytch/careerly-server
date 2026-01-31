package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	userCachePrefix   = "user:"
	userListCacheKey  = "users:list"
	userCacheDuration = 15 * time.Minute
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrForbiddenAction = errors.New("only admin can perform this action")
)

type userService struct {
	userRepo  domain.UserRepository
	cacheRepo domain.CacheRepository
}

func NewUserService(userRepo domain.UserRepository, cacheRepo domain.CacheRepository) domain.UserService {
	return &userService{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
	}
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, id.String())

	cached, err := s.cacheRepo.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		var user domain.User
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	_ = s.cacheRepo.Set(ctx, cacheKey, user, userCacheDuration)

	return user, nil
}

func (s *userService) GetAll(ctx context.Context, page, limit int) (*domain.PaginatedUsers, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	users, err := s.userRepo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedUsers{
		Users: users,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *userService) Update(ctx context.Context, id uuid.UUID, name string, avatarURL *string) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user.Name = name
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, id.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, userListCacheKey+"*")

	return user, nil
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID, requestingUserRole domain.Role) error {
	if requestingUserRole != domain.RoleAdmin {
		return ErrForbiddenAction
	}

	_, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	if err := s.userRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, id.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, userListCacheKey+"*")

	return nil
}
