package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	userListCacheKey = "users:list"
)

var (
	ErrForbiddenAction = errors.New("only admin can perform this action")
)

type userService struct {
	userRepo         domain.UserRepository
	cacheRepo        domain.CacheRepository
	subscriptionRepo domain.SubscriptionRepository
	usageRepo        domain.UsageRepository
}

func NewUserService(userRepo domain.UserRepository, cacheRepo domain.CacheRepository, subscriptionRepo domain.SubscriptionRepository, usageRepo domain.UsageRepository) domain.UserService {
	return &userService{
		userRepo:         userRepo,
		cacheRepo:        cacheRepo,
		subscriptionRepo: subscriptionRepo,
		usageRepo:        usageRepo,
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
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	_ = s.cacheRepo.Set(ctx, cacheKey, user, userCacheDuration)

	return user, nil
}

func (s *userService) GetProfile(ctx context.Context, id uuid.UUID) (*domain.UserProfileResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	var subscription *domain.Subscription
	sub, err := s.subscriptionRepo.FindActiveByUserID(ctx, id)
	if err == nil {
		subscription = sub
	}

	usages, err := s.usageRepo.GetAllCurrentMonthUsage(ctx, id)
	if err != nil {
		usages = []domain.Usage{}
	}

	return &domain.UserProfileResponse{
		User:         *user,
		Subscription: subscription,
		Usage:        usages,
	}, nil
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

func (s *userService) Update(ctx context.Context, id uuid.UUID, name string) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	user.Name = name

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, id.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, userListCacheKey+"*")

	return user, nil
}

func (s *userService) UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	if err := s.userRepo.UpdateAvatar(ctx, id, avatarURL); err != nil {
		return nil, err
	}

	user.AvatarURL = &avatarURL

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
			return domain.ErrUserNotFound
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
