package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type InterviewStatus string

const (
	InterviewStatusInProgress InterviewStatus = "in_progress"
	InterviewStatusCompleted  InterviewStatus = "completed"
	InterviewStatusCanceled   InterviewStatus = "canceled"
)

type QuestionType string

const (
	QuestionTypeEssay          QuestionType = "essay"
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
)

type Question struct {
	ID            int          `json:"id"`
	Type          QuestionType `json:"type"`
	Question      string       `json:"question"`
	Options       []Option     `json:"options,omitempty"`
	CorrectAnswer string       `json:"-"`
	UserAnswer    string       `json:"user_answer,omitempty"`
	IsCorrect     *bool        `json:"is_correct,omitempty"`
	Score         *float64     `json:"score,omitempty"`
	Feedback      string       `json:"feedback,omitempty"`
}

type Option struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type Interview struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	JobPosition  string          `json:"job_position"`
	Questions    []Question      `json:"questions"`
	Status       InterviewStatus `json:"status"`
	OverallScore *float64        `json:"overall_score,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	DeletedAt    *time.Time      `json:"deleted_at,omitempty"`
}

type InterviewForUser struct {
	ID           uuid.UUID         `json:"id"`
	UserID       uuid.UUID         `json:"user_id"`
	JobPosition  string            `json:"job_position"`
	Questions    []QuestionForUser `json:"questions"`
	Status       InterviewStatus   `json:"status"`
	OverallScore *float64          `json:"overall_score,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

type QuestionForUser struct {
	ID         int          `json:"id"`
	Type       QuestionType `json:"type"`
	Question   string       `json:"question"`
	Options    []Option     `json:"options,omitempty"`
	UserAnswer string       `json:"user_answer,omitempty"`
	IsCorrect  *bool        `json:"is_correct,omitempty"`
	Score      *float64     `json:"score,omitempty"`
	Feedback   string       `json:"feedback,omitempty"`
}

type CreateInterviewRequest struct {
	JobPosition   string       `json:"job_position" validate:"required,min=3,max=255"`
	QuestionType  QuestionType `json:"question_type" validate:"required,oneof=essay multiple_choice"`
	QuestionCount int          `json:"question_count" validate:"required,min=1,max=20"`
}

type SubmitAnswerRequest struct {
	Answers []AnswerSubmission `json:"answers" validate:"required,dive"`
}

type AnswerSubmission struct {
	QuestionID int    `json:"question_id" validate:"required,min=1"`
	Answer     string `json:"answer" validate:"required"`
}

type PaginatedInterviews struct {
	Interviews []InterviewForUser `json:"interviews"`
	Pagination Pagination         `json:"pagination"`
}

type InterviewResponse struct {
	Interview          *InterviewForUser `json:"interview"`
	AIGenerationStatus string            `json:"ai_generation_status,omitempty"`
	AIEvaluationStatus string            `json:"ai_evaluation_status,omitempty"`
}

type InterviewRepository interface {
	Create(ctx context.Context, interview *Interview) error
	FindByID(ctx context.Context, id uuid.UUID) (*Interview, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Interview, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	Update(ctx context.Context, interview *Interview) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type InterviewService interface {
	Create(ctx context.Context, userID uuid.UUID, req *CreateInterviewRequest) (*InterviewResponse, error)
	GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*InterviewForUser, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*PaginatedInterviews, error)
	SubmitAnswers(ctx context.Context, userID uuid.UUID, id uuid.UUID, req *SubmitAnswerRequest) (*InterviewResponse, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}
