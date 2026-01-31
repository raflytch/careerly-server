package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"

	"github.com/google/uuid"
)

const (
	userColumns = `id, google_id, email, name, avatar_url, role, is_active, created_at, last_login_at, deleted_at`
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, google_id, email, name, avatar_url, role, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.GoogleID,
		user.Email,
		user.Name,
		user.AvatarURL,
		user.Role,
		user.IsActive,
		user.CreatedAt,
	)
	return err
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanUser(r.db.QueryRowContext(ctx, query, id))
}

func (r *userRepository) FindByGoogleID(ctx context.Context, googleID string) (*domain.User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE google_id = $1 AND deleted_at IS NULL
	`
	return r.scanUser(r.db.QueryRowContext(ctx, query, googleID))
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	return r.scanUser(r.db.QueryRowContext(ctx, query, email))
}

func (r *userRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *userRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(id) FROM users WHERE deleted_at IS NULL`
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET name = $1, avatar_url = $2
		WHERE id = $3 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, user.Name, user.AvatarURL, user.ID)
	return err
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET last_login_at = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *userRepository) scanUser(row *sql.Row) (*domain.User, error) {
	var user domain.User
	var role string
	err := row.Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&role,
		&user.IsActive,
		&user.CreatedAt,
		&user.LastLoginAt,
		&user.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	user.Role = domain.Role(role)
	return &user, nil
}

func (r *userRepository) scanUserFromRows(rows *sql.Rows) (*domain.User, error) {
	var user domain.User
	var role string
	err := rows.Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&role,
		&user.IsActive,
		&user.CreatedAt,
		&user.LastLoginAt,
		&user.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	user.Role = domain.Role(role)
	return &user, nil
}
