package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	planCachePrefix  = "plan:"
	planListCacheKey = "plans:list"
)

var (
	ErrPlanNotFound    = errors.New("plan not found")
	ErrPlanNameExists  = errors.New("plan name already exists")
	ErrInvalidPlanData = errors.New("invalid plan data")
)

type planService struct {
	planRepo  domain.PlanRepository
	cacheRepo domain.CacheRepository
}

func NewPlanService(planRepo domain.PlanRepository, cacheRepo domain.CacheRepository) domain.PlanService {
	return &planService{
		planRepo:  planRepo,
		cacheRepo: cacheRepo,
	}
}

func (s *planService) Create(ctx context.Context, req *domain.CreatePlanRequest) (*domain.Plan, error) {
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	existing, _ := s.planRepo.FindByName(ctx, req.Name)
	if existing != nil {
		return nil, ErrPlanNameExists
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	plan := &domain.Plan{
		ID:            uuid.New(),
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		Price:         req.Price,
		DurationDays:  req.DurationDays,
		MaxResumes:    req.MaxResumes,
		MaxATSChecks:  req.MaxATSChecks,
		MaxInterviews: req.MaxInterviews,
		IsActive:      isActive,
		CreatedAt:     time.Now(),
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}

	s.invalidateListCache(ctx)

	return plan, nil
}

func (s *planService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	plan, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	return plan, nil
}

func (s *planService) GetAll(ctx context.Context, page, limit int, includeInactive bool) (*domain.PaginatedPlans, error) {
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

	total, err := s.planRepo.Count(ctx, includeInactive)
	if err != nil {
		return nil, err
	}

	plans, err := s.planRepo.FindAll(ctx, limit, offset, includeInactive)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedPlans{
		Plans: plans,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *planService) Update(ctx context.Context, id uuid.UUID, req *domain.UpdatePlanRequest) (*domain.Plan, error) {
	plan, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if req.Name != nil && *req.Name != plan.Name {
		existing, _ := s.planRepo.FindByName(ctx, *req.Name)
		if existing != nil {
			return nil, ErrPlanNameExists
		}
		plan.Name = *req.Name
	}

	if req.DisplayName != nil {
		plan.DisplayName = *req.DisplayName
	}
	if req.Price != nil {
		plan.Price = *req.Price
	}
	if req.DurationDays != nil {
		plan.DurationDays = req.DurationDays
	}
	if req.MaxResumes != nil {
		plan.MaxResumes = req.MaxResumes
	}
	if req.MaxATSChecks != nil {
		plan.MaxATSChecks = req.MaxATSChecks
	}
	if req.MaxInterviews != nil {
		plan.MaxInterviews = req.MaxInterviews
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, err
	}

	s.invalidateCache(ctx, id)

	return plan, nil
}

func (s *planService) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPlanNotFound
		}
		return err
	}

	if err := s.planRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	s.invalidateCache(ctx, id)

	return nil
}

func (s *planService) validateCreateRequest(req *domain.CreatePlanRequest) error {
	if req.Name == "" {
		return ErrInvalidPlanData
	}
	if req.DisplayName == "" {
		return ErrInvalidPlanData
	}
	return nil
}

func (s *planService) invalidateCache(ctx context.Context, id uuid.UUID) {
	cacheKey := fmt.Sprintf("%s%s", planCachePrefix, id.String())
	_ = s.cacheRepo.Delete(ctx, cacheKey)
	s.invalidateListCache(ctx)
}

func (s *planService) invalidateListCache(ctx context.Context) {
	_ = s.cacheRepo.DeleteByPattern(ctx, planListCacheKey+"*")
}
