package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	atsCheckColumns = `id, user_id, score, analysis, created_at, deleted_at`
)

type atsCheckRepository struct {
	db *sql.DB
}

func NewATSCheckRepository(db *sql.DB) domain.ATSCheckRepository {
	return &atsCheckRepository{db: db}
}

func (r *atsCheckRepository) Create(ctx context.Context, check *domain.ATSCheck) error {
	analysisJSON, err := json.Marshal(check.Analysis)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ats_checks (id, user_id, score, analysis, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.db.ExecContext(ctx, query,
		check.ID,
		check.UserID,
		check.Score,
		analysisJSON,
		check.CreatedAt,
	)
	return err
}

func (r *atsCheckRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ATSCheck, error) {
	query := `
		SELECT ` + atsCheckColumns + `
		FROM ats_checks
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanATSCheck(r.db.QueryRowContext(ctx, query, id))
}

func (r *atsCheckRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.ATSCheck, error) {
	query := `
		SELECT ` + atsCheckColumns + `
		FROM ats_checks
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checks := make([]domain.ATSCheck, 0)
	for rows.Next() {
		check, err := r.scanATSCheckFromRows(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, *check)
	}
	return checks, rows.Err()
}

func (r *atsCheckRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(id) FROM ats_checks WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

func (r *atsCheckRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE ats_checks
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *atsCheckRepository) scanATSCheck(row *sql.Row) (*domain.ATSCheck, error) {
	var check domain.ATSCheck
	var analysisJSON []byte

	err := row.Scan(
		&check.ID,
		&check.UserID,
		&check.Score,
		&analysisJSON,
		&check.CreatedAt,
		&check.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if analysisJSON != nil {
		var analysis domain.ATSAnalysis
		if err := json.Unmarshal(analysisJSON, &analysis); err != nil {
			return nil, err
		}
		check.Analysis = &analysis
	}

	return &check, nil
}

func (r *atsCheckRepository) scanATSCheckFromRows(rows *sql.Rows) (*domain.ATSCheck, error) {
	var check domain.ATSCheck
	var analysisJSON []byte

	err := rows.Scan(
		&check.ID,
		&check.UserID,
		&check.Score,
		&analysisJSON,
		&check.CreatedAt,
		&check.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if analysisJSON != nil {
		var analysis domain.ATSAnalysis
		if err := json.Unmarshal(analysisJSON, &analysis); err != nil {
			return nil, err
		}
		check.Analysis = &analysis
	}

	return &check, nil
}
