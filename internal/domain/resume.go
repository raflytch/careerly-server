package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
)

type Subscription struct {
	ID        uuid.UUID          `json:"id"`
	UserID    uuid.UUID          `json:"user_id"`
	PlanID    uuid.UUID          `json:"plan_id"`
	StartDate time.Time          `json:"start_date"`
	EndDate   time.Time          `json:"end_date"`
	Status    SubscriptionStatus `json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	DeletedAt *time.Time         `json:"deleted_at,omitempty"`
	Plan      *Plan              `json:"plan,omitempty"`
}

type SubscriptionRepository interface {
	Create(ctx context.Context, subscription *Subscription) error
	FindByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) (*Subscription, error)
	FindAllByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Subscription, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	Update(ctx context.Context, subscription *Subscription) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type FeatureType string

const (
	FeatureResume    FeatureType = "resume"
	FeatureATSCheck  FeatureType = "ats_check"
	FeatureInterview FeatureType = "interview"
)

type Usage struct {
	ID          uuid.UUID   `json:"id"`
	UserID      uuid.UUID   `json:"user_id"`
	Feature     FeatureType `json:"feature"`
	PeriodMonth time.Time   `json:"period_month"`
	Count       int         `json:"count"`
	CreatedAt   time.Time   `json:"created_at"`
	DeletedAt   *time.Time  `json:"deleted_at,omitempty"`
}

type UsageRepository interface {
	FindOrCreate(ctx context.Context, userID uuid.UUID, feature FeatureType, periodMonth time.Time) (*Usage, error)
	IncrementCount(ctx context.Context, id uuid.UUID) error
	GetCurrentMonthUsage(ctx context.Context, userID uuid.UUID, feature FeatureType) (*Usage, error)
}

type ResumeContent struct {
	PersonalInfo PersonalInfo `json:"personal_info"`
	Summary      string       `json:"summary"`
	Experience   []Experience `json:"experience"`
	Education    []Education  `json:"education"`
	Skills       []string     `json:"skills"`
	Achievements []string     `json:"achievements,omitempty"`
	Volunteer    []Volunteer  `json:"volunteer,omitempty"`
	Languages    []Language   `json:"languages,omitempty"`
	Hobbies      []string     `json:"hobbies,omitempty"`
}

type PersonalInfo struct {
	FullName    string `json:"full_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Location    string `json:"location"`
	LinkedIn    string `json:"linkedin,omitempty"`
	Portfolio   string `json:"portfolio,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
}

type Experience struct {
	Company     string `json:"company"`
	Position    string `json:"position"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
}

type Education struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Field       string `json:"field"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	GPA         string `json:"gpa,omitempty"`
	Location    string `json:"location,omitempty"`
}

type Volunteer struct {
	Organization string `json:"organization"`
	Role         string `json:"role"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Description  string `json:"description"`
}

type Language struct {
	Name        string `json:"name"`
	Proficiency string `json:"proficiency"`
}

type Resume struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"user_id"`
	Title     string        `json:"title"`
	Content   ResumeContent `json:"content"`
	IsActive  bool          `json:"is_active"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	DeletedAt *time.Time    `json:"deleted_at,omitempty"`
}

type ResumeRaw struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"user_id"`
	Title     string          `json:"title"`
	Content   json.RawMessage `json:"content"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

type CreateResumeRequest struct {
	Title        string       `json:"title" validate:"required,min=3,max=255"`
	PersonalInfo PersonalInfo `json:"personal_info" validate:"required"`
	Summary      string       `json:"summary" validate:"omitempty"`
	Experience   []Experience `json:"experience" validate:"omitempty,dive"`
	Education    []Education  `json:"education" validate:"omitempty,dive"`
	Skills       []string     `json:"skills" validate:"omitempty"`
	Achievements []string     `json:"achievements" validate:"omitempty"`
	Volunteer    []Volunteer  `json:"volunteer" validate:"omitempty,dive"`
	Languages    []Language   `json:"languages" validate:"omitempty,dive"`
	Hobbies      []string     `json:"hobbies" validate:"omitempty"`
}

type UpdateResumeRequest struct {
	Title        *string       `json:"title" validate:"omitempty,min=3,max=255"`
	PersonalInfo *PersonalInfo `json:"personal_info" validate:"omitempty"`
	Summary      *string       `json:"summary" validate:"omitempty"`
	Experience   []Experience  `json:"experience" validate:"omitempty,dive"`
	Education    []Education   `json:"education" validate:"omitempty,dive"`
	Skills       []string      `json:"skills" validate:"omitempty"`
	Achievements []string      `json:"achievements" validate:"omitempty"`
	Volunteer    []Volunteer   `json:"volunteer" validate:"omitempty,dive"`
	Languages    []Language    `json:"languages" validate:"omitempty,dive"`
	Hobbies      []string      `json:"hobbies" validate:"omitempty"`
	IsActive     *bool         `json:"is_active" validate:"omitempty"`
}

type PaginatedResumes struct {
	Resumes    []Resume   `json:"resumes"`
	Pagination Pagination `json:"pagination"`
}

type ResumeResponse struct {
	Resume             *Resume `json:"resume"`
	AIConversionStatus string  `json:"ai_conversion_status"`
}

type ResumeRepository interface {
	Create(ctx context.Context, resume *Resume) error
	FindByID(ctx context.Context, id uuid.UUID) (*Resume, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Resume, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	Update(ctx context.Context, resume *Resume) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type ResumeService interface {
	Create(ctx context.Context, userID uuid.UUID, req *CreateResumeRequest) (*ResumeResponse, error)
	GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*Resume, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*PaginatedResumes, error)
	Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, req *UpdateResumeRequest) (*ResumeResponse, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	GeneratePDF(ctx context.Context, userID uuid.UUID, id uuid.UUID) ([]byte, error)
}

type QuotaService interface {
	CheckAndIncrementUsage(ctx context.Context, userID uuid.UUID, feature FeatureType) error
	GetUserQuota(ctx context.Context, userID uuid.UUID) (*UserQuota, error)
}

type UserQuota struct {
	PlanName       string `json:"plan_name"`
	MaxResumes     int    `json:"max_resumes"`
	MaxATSChecks   int    `json:"max_ats_checks"`
	MaxInterviews  int    `json:"max_interviews"`
	UsedResumes    int    `json:"used_resumes"`
	UsedATSChecks  int    `json:"used_ats_checks"`
	UsedInterviews int    `json:"used_interviews"`
}
