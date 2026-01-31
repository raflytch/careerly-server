package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	planColumns = `id, name, display_name, price, duration_days, max_resumes, max_ats_checks, max_interviews, is_active, created_at, deleted_at`
)

type planRepository struct {
	db *sql.DB
}

func NewPlanRepository(db *sql.DB) domain.PlanRepository {
	return &planRepository{db: db}
}

func (r *planRepository) Create(ctx context.Context, plan *domain.Plan) error {
	query := `
		INSERT INTO plans (id, name, display_name, price, duration_days, max_resumes, max_ats_checks, max_interviews, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		plan.ID,
		plan.Name,
		plan.DisplayName,
		plan.Price,
		plan.DurationDays,
		plan.MaxResumes,
		plan.MaxATSChecks,
		plan.MaxInterviews,
		plan.IsActive,
		plan.CreatedAt,
	)
	return err
}

func (r *planRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	query := `
		SELECT ` + planColumns + `
		FROM plans
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanPlan(r.db.QueryRowContext(ctx, query, id))
}

func (r *planRepository) FindByName(ctx context.Context, name string) (*domain.Plan, error) {
	query := `
		SELECT ` + planColumns + `
		FROM plans
		WHERE name = $1 AND deleted_at IS NULL
	`
	return r.scanPlan(r.db.QueryRowContext(ctx, query, name))
}

func (r *planRepository) FindAll(ctx context.Context, limit, offset int, includeInactive bool) ([]domain.Plan, error) {
	query := `
		SELECT ` + planColumns + `
		FROM plans
		WHERE deleted_at IS NULL
	`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += `
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := make([]domain.Plan, 0)
	for rows.Next() {
		plan, err := r.scanPlanFromRows(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, *plan)
	}
	return plans, rows.Err()
}

func (r *planRepository) Count(ctx context.Context, includeInactive bool) (int64, error) {
	query := `SELECT COUNT(id) FROM plans WHERE deleted_at IS NULL`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *planRepository) Update(ctx context.Context, plan *domain.Plan) error {
	query := `
		UPDATE plans
		SET name = $1, display_name = $2, price = $3, duration_days = $4, 
			max_resumes = $5, max_ats_checks = $6, max_interviews = $7, is_active = $8
		WHERE id = $9 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query,
		plan.Name,
		plan.DisplayName,
		plan.Price,
		plan.DurationDays,
		plan.MaxResumes,
		plan.MaxATSChecks,
		plan.MaxInterviews,
		plan.IsActive,
		plan.ID,
	)
	return err
}

func (r *planRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE plans
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *planRepository) scanPlan(row *sql.Row) (*domain.Plan, error) {
	var plan domain.Plan
	var price decimal.Decimal
	err := row.Scan(
		&plan.ID,
		&plan.Name,
		&plan.DisplayName,
		&price,
		&plan.DurationDays,
		&plan.MaxResumes,
		&plan.MaxATSChecks,
		&plan.MaxInterviews,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	plan.Price = price
	return &plan, nil
}

func (r *planRepository) scanPlanFromRows(rows *sql.Rows) (*domain.Plan, error) {
	var plan domain.Plan
	var price decimal.Decimal
	err := rows.Scan(
		&plan.ID,
		&plan.Name,
		&plan.DisplayName,
		&price,
		&plan.DurationDays,
		&plan.MaxResumes,
		&plan.MaxATSChecks,
		&plan.MaxInterviews,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	plan.Price = price
	return &plan, nil
}
