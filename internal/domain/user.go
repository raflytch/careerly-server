package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	GoogleID    string     `json:"google_id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	AvatarURL   *string    `json:"avatar_url"`
	Role        Role       `json:"role"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type PaginatedUsers struct {
	Users      []User     `json:"users"`
	Pagination Pagination `json:"pagination"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByGoogleID(ctx context.Context, googleID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindAll(ctx context.Context, limit, offset int) ([]User, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, user *User) error
	UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

type CacheRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
	DeleteByPattern(ctx context.Context, pattern string) error
}

type UserService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetAll(ctx context.Context, page, limit int) (*PaginatedUsers, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*User, error)
	UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) (*User, error)
	Delete(ctx context.Context, id uuid.UUID, requestingUserRole Role) error
}

type AuthService interface {
	GetGoogleLoginURL(state string) string
	HandleGoogleCallback(ctx context.Context, code string) (*AuthResponse, error)
	ValidateToken(ctx context.Context, tokenString string) (*User, error)
}
