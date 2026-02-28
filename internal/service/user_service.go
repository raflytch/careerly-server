package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	userListCacheKey  = "users:list"
	deleteOTPPrefix   = "otp:delete:"
	deleteOTPDuration = userCacheDuration
	deleteOTPLength   = 6
)

var (
	ErrForbiddenAction = errors.New("only admin can perform this action")
)

type userService struct {
	userRepo         domain.UserRepository
	cacheRepo        domain.CacheRepository
	subscriptionRepo domain.SubscriptionRepository
	usageRepo        domain.UsageRepository
	emailService     domain.EmailService
}

func NewUserService(userRepo domain.UserRepository, cacheRepo domain.CacheRepository, subscriptionRepo domain.SubscriptionRepository, usageRepo domain.UsageRepository, emailService domain.EmailService) domain.UserService {
	return &userService{
		userRepo:         userRepo,
		cacheRepo:        cacheRepo,
		subscriptionRepo: subscriptionRepo,
		usageRepo:        usageRepo,
		emailService:     emailService,
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

func (s *userService) RequestDeleteOTP(ctx context.Context, user *domain.User) (*domain.OTPResponse, error) {
	if user.Role == domain.RoleAdmin {
		return nil, domain.ErrCannotDeleteAdmin
	}

	otpKey := fmt.Sprintf("%s%s", deleteOTPPrefix, user.Email)
	existingOTP, err := s.cacheRepo.Get(ctx, otpKey)
	if err == nil && existingOTP != "" {
		return nil, domain.ErrOTPAlreadySent
	}

	otp, err := GenerateOTP(deleteOTPLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	if err := s.cacheRepo.Set(ctx, otpKey, otp, deleteOTPDuration); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	if err := s.emailService.SendDeleteOTP(ctx, user.Email, otp); err != nil {
		_ = s.cacheRepo.Delete(ctx, otpKey)
		return nil, fmt.Errorf("failed to send OTP email: %w", err)
	}

	return &domain.OTPResponse{
		Message:   "OTP has been sent to your email address",
		ExpiresIn: int(deleteOTPDuration.Seconds()),
	}, nil
}

func (s *userService) VerifyDeleteOTP(ctx context.Context, user *domain.User, otp string) (*domain.DeleteAccountResponse, error) {
	if user.Role == domain.RoleAdmin {
		return nil, domain.ErrCannotDeleteAdmin
	}

	otpKey := fmt.Sprintf("%s%s", deleteOTPPrefix, user.Email)
	storedOTP, err := s.cacheRepo.Get(ctx, otpKey)
	if err != nil {
		return nil, domain.ErrInvalidOTP
	}

	storedOTP = strings.Trim(storedOTP, "\"")
	if storedOTP != otp {
		return nil, domain.ErrInvalidOTP
	}

	if err := s.userRepo.SoftDelete(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to delete account: %w", err)
	}

	_ = s.cacheRepo.Delete(ctx, otpKey)

	cacheKey := fmt.Sprintf("%s%s", userCachePrefix, user.ID.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	_ = s.cacheRepo.DeleteByPattern(ctx, userListCacheKey+"*")

	return &domain.DeleteAccountResponse{
		Message: "your account has been successfully deleted",
	}, nil
}

func (s *userService) ResendDeleteOTP(ctx context.Context, user *domain.User) (*domain.OTPResponse, error) {
	if user.Role == domain.RoleAdmin {
		return nil, domain.ErrCannotDeleteAdmin
	}

	otpKey := fmt.Sprintf("%s%s", deleteOTPPrefix, user.Email)
	_ = s.cacheRepo.Delete(ctx, otpKey)

	otp, err := GenerateOTP(deleteOTPLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	if err := s.cacheRepo.Set(ctx, otpKey, otp, deleteOTPDuration); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	if err := s.emailService.SendDeleteOTP(ctx, user.Email, otp); err != nil {
		_ = s.cacheRepo.Delete(ctx, otpKey)
		return nil, fmt.Errorf("failed to send OTP email: %w", err)
	}

	return &domain.OTPResponse{
		Message:   "a new OTP has been sent to your email address",
		ExpiresIn: int(deleteOTPDuration.Seconds()),
	}, nil
}
