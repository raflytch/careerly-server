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
	interviewColumns = `id, user_id, job_position, questions, status, overall_score, created_at, completed_at, deleted_at`
)

type interviewRepository struct {
	db *sql.DB
}

func NewInterviewRepository(db *sql.DB) domain.InterviewRepository {
	return &interviewRepository{db: db}
}

func (r *interviewRepository) Create(ctx context.Context, interview *domain.Interview) error {
	questionsJSON, err := json.Marshal(interview.Questions)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO interviews (id, user_id, job_position, questions, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.db.ExecContext(ctx, query,
		interview.ID,
		interview.UserID,
		interview.JobPosition,
		questionsJSON,
		interview.Status,
		interview.CreatedAt,
	)
	return err
}

func (r *interviewRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Interview, error) {
	query := `
		SELECT ` + interviewColumns + `
		FROM interviews
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanInterview(r.db.QueryRowContext(ctx, query, id))
}

func (r *interviewRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Interview, error) {
	query := `
		SELECT ` + interviewColumns + `
		FROM interviews
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	interviews := make([]domain.Interview, 0)
	for rows.Next() {
		interview, err := r.scanInterviewFromRows(rows)
		if err != nil {
			return nil, err
		}
		interviews = append(interviews, *interview)
	}
	return interviews, rows.Err()
}

func (r *interviewRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(id) FROM interviews WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

func (r *interviewRepository) Update(ctx context.Context, interview *domain.Interview) error {
	questionsJSON, err := json.Marshal(interview.Questions)
	if err != nil {
		return err
	}

	query := `
		UPDATE interviews
		SET questions = $1, status = $2, overall_score = $3, completed_at = $4
		WHERE id = $5 AND deleted_at IS NULL
	`
	_, err = r.db.ExecContext(ctx, query,
		questionsJSON,
		interview.Status,
		interview.OverallScore,
		interview.CompletedAt,
		interview.ID,
	)
	return err
}

func (r *interviewRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE interviews
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *interviewRepository) scanInterview(row *sql.Row) (*domain.Interview, error) {
	var interview domain.Interview
	var questionsJSON []byte
	var status string
	err := row.Scan(
		&interview.ID,
		&interview.UserID,
		&interview.JobPosition,
		&questionsJSON,
		&status,
		&interview.OverallScore,
		&interview.CreatedAt,
		&interview.CompletedAt,
		&interview.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	interview.Status = domain.InterviewStatus(status)

	if err := json.Unmarshal(questionsJSON, &interview.Questions); err != nil {
		return nil, err
	}

	return &interview, nil
}

func (r *interviewRepository) scanInterviewFromRows(rows *sql.Rows) (*domain.Interview, error) {
	var interview domain.Interview
	var questionsJSON []byte
	var status string
	err := rows.Scan(
		&interview.ID,
		&interview.UserID,
		&interview.JobPosition,
		&questionsJSON,
		&status,
		&interview.OverallScore,
		&interview.CreatedAt,
		&interview.CompletedAt,
		&interview.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	interview.Status = domain.InterviewStatus(status)

	if err := json.Unmarshal(questionsJSON, &interview.Questions); err != nil {
		return nil, err
	}

	return &interview, nil
}
