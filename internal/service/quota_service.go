package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

var (
	ErrNoActiveSubscription = errors.New("no active subscription found")
	ErrQuotaExceeded        = errors.New("quota exceeded for this feature")
)

type quotaService struct {
	subscriptionRepo domain.SubscriptionRepository
	usageRepo        domain.UsageRepository
}

func NewQuotaService(subscriptionRepo domain.SubscriptionRepository, usageRepo domain.UsageRepository) domain.QuotaService {
	return &quotaService{
		subscriptionRepo: subscriptionRepo,
		usageRepo:        usageRepo,
	}
}

func (s *quotaService) CheckAndIncrementUsage(ctx context.Context, userID uuid.UUID, feature domain.FeatureType) error {
	subscription, err := s.subscriptionRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoActiveSubscription
		}
		return err
	}

	if subscription.Plan == nil {
		return ErrNoActiveSubscription
	}

	now := time.Now()
	periodMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	usage, err := s.usageRepo.FindOrCreate(ctx, userID, feature, periodMonth)
	if err != nil {
		return err
	}

	var maxAllowed int
	switch feature {
	case domain.FeatureResume:
		if subscription.Plan.MaxResumes != nil {
			maxAllowed = *subscription.Plan.MaxResumes
		}
	case domain.FeatureATSCheck:
		if subscription.Plan.MaxATSChecks != nil {
			maxAllowed = *subscription.Plan.MaxATSChecks
		}
	case domain.FeatureInterview:
		if subscription.Plan.MaxInterviews != nil {
			maxAllowed = *subscription.Plan.MaxInterviews
		}
	}

	if maxAllowed > 0 && usage.Count >= maxAllowed {
		return ErrQuotaExceeded
	}

	return s.usageRepo.IncrementCount(ctx, usage.ID)
}

func (s *quotaService) GetUserQuota(ctx context.Context, userID uuid.UUID) (*domain.UserQuota, error) {
	subscription, err := s.subscriptionRepo.FindActiveByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoActiveSubscription
		}
		return nil, err
	}

	if subscription.Plan == nil {
		return nil, ErrNoActiveSubscription
	}

	now := time.Now()
	periodMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	resumeUsage, _ := s.usageRepo.FindOrCreate(ctx, userID, domain.FeatureResume, periodMonth)
	atsUsage, _ := s.usageRepo.FindOrCreate(ctx, userID, domain.FeatureATSCheck, periodMonth)
	interviewUsage, _ := s.usageRepo.FindOrCreate(ctx, userID, domain.FeatureInterview, periodMonth)

	quota := &domain.UserQuota{
		PlanName: subscription.Plan.DisplayName,
	}

	if subscription.Plan.MaxResumes != nil {
		quota.MaxResumes = *subscription.Plan.MaxResumes
	}
	if subscription.Plan.MaxATSChecks != nil {
		quota.MaxATSChecks = *subscription.Plan.MaxATSChecks
	}
	if subscription.Plan.MaxInterviews != nil {
		quota.MaxInterviews = *subscription.Plan.MaxInterviews
	}

	if resumeUsage != nil {
		quota.UsedResumes = resumeUsage.Count
	}
	if atsUsage != nil {
		quota.UsedATSChecks = atsUsage.Count
	}
	if interviewUsage != nil {
		quota.UsedInterviews = interviewUsage.Count
	}

	return quota, nil
}
