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
	resumeColumns = `id, user_id, title, content, is_active, created_at, updated_at, deleted_at`
)

type resumeRepository struct {
	db *sql.DB
}

func NewResumeRepository(db *sql.DB) domain.ResumeRepository {
	return &resumeRepository{db: db}
}

func (r *resumeRepository) Create(ctx context.Context, resume *domain.Resume) error {
	contentJSON, err := json.Marshal(resume.Content)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO resumes (id, user_id, title, content, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.ExecContext(ctx, query,
		resume.ID,
		resume.UserID,
		resume.Title,
		contentJSON,
		resume.IsActive,
		resume.CreatedAt,
		resume.UpdatedAt,
	)
	return err
}

func (r *resumeRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Resume, error) {
	query := `
		SELECT ` + resumeColumns + `
		FROM resumes
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanResume(r.db.QueryRowContext(ctx, query, id))
}

func (r *resumeRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Resume, error) {
	query := `
		SELECT ` + resumeColumns + `
		FROM resumes
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resumes := make([]domain.Resume, 0)
	for rows.Next() {
		resume, err := r.scanResumeFromRows(rows)
		if err != nil {
			return nil, err
		}
		resumes = append(resumes, *resume)
	}
	return resumes, rows.Err()
}

func (r *resumeRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(id) FROM resumes WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

func (r *resumeRepository) Update(ctx context.Context, resume *domain.Resume) error {
	contentJSON, err := json.Marshal(resume.Content)
	if err != nil {
		return err
	}

	query := `
		UPDATE resumes
		SET title = $1, content = $2, is_active = $3, updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
	`
	_, err = r.db.ExecContext(ctx, query,
		resume.Title,
		contentJSON,
		resume.IsActive,
		time.Now(),
		resume.ID,
	)
	return err
}

func (r *resumeRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE resumes
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *resumeRepository) scanResume(row *sql.Row) (*domain.Resume, error) {
	var resume domain.Resume
	var contentJSON []byte
	err := row.Scan(
		&resume.ID,
		&resume.UserID,
		&resume.Title,
		&contentJSON,
		&resume.IsActive,
		&resume.CreatedAt,
		&resume.UpdatedAt,
		&resume.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &resume.Content); err != nil {
		return nil, err
	}

	return &resume, nil
}

func (r *resumeRepository) scanResumeFromRows(rows *sql.Rows) (*domain.Resume, error) {
	var resume domain.Resume
	var contentJSON []byte
	err := rows.Scan(
		&resume.ID,
		&resume.UserID,
		&resume.Title,
		&contentJSON,
		&resume.IsActive,
		&resume.CreatedAt,
		&resume.UpdatedAt,
		&resume.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(contentJSON, &resume.Content); err != nil {
		return nil, err
	}

	return &resume, nil
}
