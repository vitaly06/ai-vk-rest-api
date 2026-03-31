package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetByVKID(ctx context.Context, vkID int64) (*models.User, error) {
	u := &models.User{}
	err := r.db.GetContext(ctx, u, `SELECT * FROM users WHERE vk_id = $1`, vkID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) Create(ctx context.Context, u *models.User) (*models.User, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (vk_id, first_name, last_name, username, role, status, state, invite_id, request_limit)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at, updated_at`,
		u.VKID, u.FirstName, u.LastName, u.Username,
		u.Role, u.Status, u.State, u.InviteID, u.RequestLimit,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *UserRepo) Update(ctx context.Context, u *models.User) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET first_name=$1, last_name=$2, username=$3, role=$4,
		status=$5, state=$6, consent_given=$7, mailing_consent=$8,
		invite_id=$9, request_limit=$10, updated_at=NOW()
		WHERE vk_id=$11`,
		u.FirstName, u.LastName, u.Username, u.Role,
		u.Status, u.State, u.ConsentGiven, u.MailingConsent,
		u.InviteID, u.RequestLimit, u.VKID,
	)
	return err
}

func (r *UserRepo) UpdateState(ctx context.Context, vkID int64, state models.BotState) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET state=$1, updated_at=NOW() WHERE vk_id=$2`, state, vkID)
	return err
}

func (r *UserRepo) UpdateStatus(ctx context.Context, vkID int64, status models.UserStatus, until *time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET status=$1, banned_until=$2, updated_at=NOW() WHERE vk_id=$3`,
		status, until, vkID)
	return err
}

func (r *UserRepo) UpdateRole(ctx context.Context, vkID int64, role models.Role) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET role=$1, updated_at=NOW() WHERE vk_id=$2`, role, vkID)
	return err
}

func (r *UserRepo) UpdateBalance(ctx context.Context, vkID int64, delta float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET balance = balance + $1, updated_at=NOW() WHERE vk_id=$2`, delta, vkID)
	return err
}

func (r *UserRepo) IncrementRequestCount(ctx context.Context, vkID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET request_count = request_count + 1, updated_at=NOW() WHERE vk_id=$1`, vkID)
	return err
}

func (r *UserRepo) SetRequestLimit(ctx context.Context, vkID int64, limit int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET request_limit=$1, updated_at=NOW() WHERE vk_id=$2`, limit, vkID)
	return err
}

func (r *UserRepo) ListAll(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY created_at DESC`)
	return users, err
}

func (r *UserRepo) ListByRole(ctx context.Context, role models.Role) ([]*models.User, error) {
	var users []*models.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users WHERE role=$1 ORDER BY created_at DESC`, role)
	return users, err
}

func (r *UserRepo) SaveQuestionnaireAnswer(ctx context.Context, a *models.QuestionnaireAnswer) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO questionnaire_answers (user_id, question, answer)
		VALUES ($1,$2,$3) RETURNING id, created_at`,
		a.UserID, a.Question, a.Answer,
	).Scan(&a.ID, &a.CreatedAt)
	return err
}

func (r *UserRepo) GetQuestionnaireAnswers(ctx context.Context, userID int64) ([]*models.QuestionnaireAnswer, error) {
	var ans []*models.QuestionnaireAnswer
	err := r.db.SelectContext(ctx, &ans,
		`SELECT * FROM questionnaire_answers WHERE user_id=$1 ORDER BY created_at`, userID)
	return ans, err
}

func (r *UserRepo) CreateAccessRequest(ctx context.Context, req *models.AccessRequest) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO access_requests (vk_id, message, status)
		VALUES ($1,$2,'pending') RETURNING id, created_at, updated_at`,
		req.VKID, req.Message,
	).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
	return err
}

func (r *UserRepo) ListPendingRequests(ctx context.Context) ([]*models.AccessRequest, error) {
	var reqs []*models.AccessRequest
	err := r.db.SelectContext(ctx, &reqs,
		`SELECT * FROM access_requests WHERE status='pending' ORDER BY created_at`)
	return reqs, err
}

func (r *UserRepo) UpdateAccessRequest(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE access_requests SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}
