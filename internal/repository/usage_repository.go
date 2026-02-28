package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	usageColumns = `id, user_id, feature, period_month, count, created_at, deleted_at`
)

type usageRepository struct {
	db *sql.DB
}

func NewUsageRepository(db *sql.DB) domain.UsageRepository {
	return &usageRepository{db: db}
}

func (r *usageRepository) FindOrCreate(ctx context.Context, userID uuid.UUID, feature domain.FeatureType, periodMonth time.Time) (*domain.Usage, error) {
	usage, err := r.GetCurrentMonthUsage(ctx, userID, feature)
	if err == nil {
		return usage, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	newUsage := &domain.Usage{
		ID:          uuid.New(),
		UserID:      userID,
		Feature:     feature,
		PeriodMonth: periodMonth,
		Count:       0,
		CreatedAt:   time.Now(),
	}

	query := `
		INSERT INTO usage (id, user_id, feature, period_month, count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, feature, period_month) DO NOTHING
	`
	_, err = r.db.ExecContext(ctx, query,
		newUsage.ID,
		newUsage.UserID,
		newUsage.Feature,
		newUsage.PeriodMonth,
		newUsage.Count,
		newUsage.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return r.GetCurrentMonthUsage(ctx, userID, feature)
}

func (r *usageRepository) IncrementCount(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE usage
		SET count = count + 1
		WHERE id = $1 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *usageRepository) GetCurrentMonthUsage(ctx context.Context, userID uuid.UUID, feature domain.FeatureType) (*domain.Usage, error) {
	now := time.Now()
	periodMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT ` + usageColumns + `
		FROM usage
		WHERE user_id = $1 AND feature = $2 AND period_month = $3 AND deleted_at IS NULL
	`
	return r.scanUsage(r.db.QueryRowContext(ctx, query, userID, feature, periodMonth))
}

func (r *usageRepository) GetAllCurrentMonthUsage(ctx context.Context, userID uuid.UUID) ([]domain.Usage, error) {
	now := time.Now()
	periodMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT ` + usageColumns + `
		FROM usage
		WHERE user_id = $1 AND period_month = $2 AND deleted_at IS NULL
	`
	rows, err := r.db.QueryContext(ctx, query, userID, periodMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usages := make([]domain.Usage, 0)
	for rows.Next() {
		var usage domain.Usage
		var feature string
		err := rows.Scan(
			&usage.ID,
			&usage.UserID,
			&feature,
			&usage.PeriodMonth,
			&usage.Count,
			&usage.CreatedAt,
			&usage.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		usage.Feature = domain.FeatureType(feature)
		usages = append(usages, usage)
	}
	return usages, rows.Err()
}

func (r *usageRepository) scanUsage(row *sql.Row) (*domain.Usage, error) {
	var usage domain.Usage
	var feature string
	err := row.Scan(
		&usage.ID,
		&usage.UserID,
		&feature,
		&usage.PeriodMonth,
		&usage.Count,
		&usage.CreatedAt,
		&usage.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	usage.Feature = domain.FeatureType(feature)
	return &usage, nil
}
