package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	subscriptionColumns = `id, user_id, plan_id, start_date, end_date, status, created_at, deleted_at`
)

type subscriptionRepository struct {
	db *sql.DB
}

func NewSubscriptionRepository(db *sql.DB) domain.SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

func (r *subscriptionRepository) Create(ctx context.Context, subscription *domain.Subscription) error {
	query := `
		INSERT INTO subscriptions (id, user_id, plan_id, start_date, end_date, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		subscription.ID,
		subscription.UserID,
		subscription.PlanID,
		subscription.StartDate,
		subscription.EndDate,
		subscription.Status,
		subscription.CreatedAt,
	)
	return err
}

func (r *subscriptionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	query := `
		SELECT ` + subscriptionColumns + `
		FROM subscriptions
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanSubscription(r.db.QueryRowContext(ctx, query, id))
}

func (r *subscriptionRepository) FindActiveByUserID(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, s.start_date, s.end_date, s.status, s.created_at, s.deleted_at,
			   p.id, p.name, p.display_name, p.price, p.duration_days, p.max_resumes, p.max_ats_checks, p.max_interviews, p.is_active, p.created_at, p.deleted_at
		FROM subscriptions s
		JOIN plans p ON s.plan_id = p.id
		WHERE s.user_id = $1 
		  AND s.status = 'active' 
		  AND s.end_date > $2
		  AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, userID, time.Now())
	return r.scanSubscriptionWithPlan(row)
}

func (r *subscriptionRepository) FindAllByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Subscription, error) {
	query := `
		SELECT ` + subscriptionColumns + `
		FROM subscriptions
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscriptions := make([]domain.Subscription, 0)
	for rows.Next() {
		sub, err := r.scanSubscriptionFromRows(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, *sub)
	}
	return subscriptions, rows.Err()
}

func (r *subscriptionRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(id) FROM subscriptions WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

func (r *subscriptionRepository) Update(ctx context.Context, subscription *domain.Subscription) error {
	query := `
		UPDATE subscriptions
		SET status = $1, end_date = $2
		WHERE id = $3 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query,
		subscription.Status,
		subscription.EndDate,
		subscription.ID,
	)
	return err
}

func (r *subscriptionRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE subscriptions
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *subscriptionRepository) scanSubscription(row *sql.Row) (*domain.Subscription, error) {
	var sub domain.Subscription
	var status string
	err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PlanID,
		&sub.StartDate,
		&sub.EndDate,
		&status,
		&sub.CreatedAt,
		&sub.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	sub.Status = domain.SubscriptionStatus(status)
	return &sub, nil
}

func (r *subscriptionRepository) scanSubscriptionFromRows(rows *sql.Rows) (*domain.Subscription, error) {
	var sub domain.Subscription
	var status string
	err := rows.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PlanID,
		&sub.StartDate,
		&sub.EndDate,
		&status,
		&sub.CreatedAt,
		&sub.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	sub.Status = domain.SubscriptionStatus(status)
	return &sub, nil
}

func (r *subscriptionRepository) scanSubscriptionWithPlan(row *sql.Row) (*domain.Subscription, error) {
	var sub domain.Subscription
	var plan domain.Plan
	var status string
	var priceStr string

	err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PlanID,
		&sub.StartDate,
		&sub.EndDate,
		&status,
		&sub.CreatedAt,
		&sub.DeletedAt,
		&plan.ID,
		&plan.Name,
		&plan.DisplayName,
		&priceStr,
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
	sub.Status = domain.SubscriptionStatus(status)
	sub.Plan = &plan
	return &sub, nil
}
