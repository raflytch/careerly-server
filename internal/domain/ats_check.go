package domain

import (
	"context"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
)

type ATSAnalysis struct {
	OverallScore    float64          `json:"overall_score"`
	Verdict         string           `json:"verdict"`
	Sections        []ATSSection     `json:"sections"`
	KeywordAnalysis ATSKeywords      `json:"keyword_analysis"`
	Improvements    []ATSImprovement `json:"improvements"`
	DealBreakers    []string         `json:"deal_breakers,omitempty"`
}

type ATSSection struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	MaxScore float64 `json:"max_score"`
	Feedback string  `json:"feedback"`
}

type ATSKeywords struct {
	Found   []string `json:"found"`
	Missing []string `json:"missing"`
	Tip     string   `json:"tip"`
}

type ATSImprovement struct {
	Priority   string `json:"priority"`
	Category   string `json:"category"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion"`
}

type ATSCheck struct {
	ID        uuid.UUID    `json:"id"`
	UserID    uuid.UUID    `json:"user_id"`
	Score     *float64     `json:"score,omitempty"`
	Analysis  *ATSAnalysis `json:"analysis,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

type ATSCheckResponse struct {
	ATSCheck         *ATSCheck `json:"ats_check"`
	AIAnalysisStatus string    `json:"ai_analysis_status"`
}

type PaginatedATSChecks struct {
	ATSChecks  []ATSCheck `json:"ats_checks"`
	Pagination Pagination `json:"pagination"`
}

type ATSCheckRepository interface {
	Create(ctx context.Context, check *ATSCheck) error
	FindByID(ctx context.Context, id uuid.UUID) (*ATSCheck, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ATSCheck, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type ATSCheckService interface {
	AnalyzeFromFile(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*ATSCheckResponse, error)
	GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*ATSCheck, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*PaginatedATSChecks, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}
