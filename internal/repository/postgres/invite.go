package postgres

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type InviteRepo struct {
	db *sqlx.DB
}

func NewInviteRepo(db *sqlx.DB) *InviteRepo {
	return &InviteRepo{db: db}
}

func (r *InviteRepo) Create(ctx context.Context, inv *models.Invite) (*models.Invite, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO invites (token, created_by_id, max_uses, expires_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, uses_count, created_at`,
		inv.Token, inv.CreatedByID, inv.MaxUses, inv.ExpiresAt,
	).Scan(&inv.ID, &inv.UsesCount, &inv.CreatedAt)
	return inv, err
}

func (r *InviteRepo) GetByToken(ctx context.Context, token string) (*models.Invite, error) {
	inv := &models.Invite{}
	err := r.db.GetContext(ctx, inv, `SELECT * FROM invites WHERE token=$1`, token)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return inv, err
}

func (r *InviteRepo) Use(ctx context.Context, token string, usedByVKID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE invites SET uses_count = uses_count + 1, used_by_id=$1
		WHERE token=$2`,
		usedByVKID, token)
	return err
}

func (r *InviteRepo) ListByCreator(ctx context.Context, creatorID int64) ([]*models.Invite, error) {
	var invites []*models.Invite
	err := r.db.SelectContext(ctx, &invites,
		`SELECT * FROM invites WHERE created_by_id=$1 ORDER BY created_at DESC`, creatorID)
	return invites, err
}

func (r *InviteRepo) ListAll(ctx context.Context) ([]*models.Invite, error) {
	var invites []*models.Invite
	err := r.db.SelectContext(ctx, &invites,
		`SELECT * FROM invites ORDER BY created_at DESC`)
	return invites, err
}
