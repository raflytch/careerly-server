package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Plan struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name"`
	Price         decimal.Decimal `json:"price"`
	DurationDays  *int            `json:"duration_days"`
	MaxResumes    *int            `json:"max_resumes"`
	MaxATSChecks  *int            `json:"max_ats_checks"`
	MaxInterviews *int            `json:"max_interviews"`
	IsActive      bool            `json:"is_active"`
	CreatedAt     time.Time       `json:"created_at"`
	DeletedAt     *time.Time      `json:"deleted_at,omitempty"`
}

type CreatePlanRequest struct {
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name"`
	Price         decimal.Decimal `json:"price"`
	DurationDays  *int            `json:"duration_days"`
	MaxResumes    *int            `json:"max_resumes"`
	MaxATSChecks  *int            `json:"max_ats_checks"`
	MaxInterviews *int            `json:"max_interviews"`
	IsActive      *bool           `json:"is_active"`
}

type UpdatePlanRequest struct {
	Name          *string          `json:"name"`
	DisplayName   *string          `json:"display_name"`
	Price         *decimal.Decimal `json:"price"`
	DurationDays  *int             `json:"duration_days"`
	MaxResumes    *int             `json:"max_resumes"`
	MaxATSChecks  *int             `json:"max_ats_checks"`
	MaxInterviews *int             `json:"max_interviews"`
	IsActive      *bool            `json:"is_active"`
}

type PaginatedPlans struct {
	Plans      []Plan     `json:"plans"`
	Pagination Pagination `json:"pagination"`
}

type PlanRepository interface {
	Create(ctx context.Context, plan *Plan) error
	FindByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	FindByName(ctx context.Context, name string) (*Plan, error)
	FindAll(ctx context.Context, limit, offset int, includeInactive bool) ([]Plan, error)
	Count(ctx context.Context, includeInactive bool) (int64, error)
	Update(ctx context.Context, plan *Plan) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type PlanService interface {
	Create(ctx context.Context, req *CreatePlanRequest) (*Plan, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	GetAll(ctx context.Context, page, limit int, includeInactive bool) (*PaginatedPlans, error)
	Update(ctx context.Context, id uuid.UUID, req *UpdatePlanRequest) (*Plan, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
