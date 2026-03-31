package postgres

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type DialogRepo struct {
	db *sqlx.DB
}

func NewDialogRepo(db *sqlx.DB) *DialogRepo {
	return &DialogRepo{db: db}
}

func (r *DialogRepo) GetOrCreateDialog(ctx context.Context, userID int64, dtype models.DialogType) (*models.Dialog, error) {
	d := &models.Dialog{}
	err := r.db.GetContext(ctx, d,
		`SELECT * FROM dialogs WHERE user_id=$1 AND type=$2 AND is_active=true LIMIT 1`,
		userID, dtype)
	if err == sql.ErrNoRows {
		err = r.db.QueryRowContext(ctx,
			`INSERT INTO dialogs (user_id, type) VALUES ($1,$2) RETURNING id, is_active, created_at, updated_at`,
			userID, dtype,
		).Scan(&d.ID, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
		d.UserID = userID
		d.Type = dtype
	}
	return d, err
}

func (r *DialogRepo) GetDialog(ctx context.Context, id int64) (*models.Dialog, error) {
	d := &models.Dialog{}
	err := r.db.GetContext(ctx, d, `SELECT * FROM dialogs WHERE id=$1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (r *DialogRepo) SaveMessage(ctx context.Context, m *models.Message) (*models.Message, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO messages (dialog_id, user_id, role, type, content, vk_message_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at`,
		m.DialogID, m.UserID, m.Role, m.Type, m.Content, m.VKMessageID,
	).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (r *DialogRepo) GetHistory(ctx context.Context, dialogID int64, limit int) ([]*models.Message, error) {
	var msgs []*models.Message
	err := r.db.SelectContext(ctx, &msgs, `
		SELECT * FROM (
			SELECT * FROM messages
			WHERE dialog_id=$1 AND is_deleted=false
			ORDER BY created_at DESC
			LIMIT $2
		) sub ORDER BY created_at ASC`,
		dialogID, limit)
	return msgs, err
}

func (r *DialogRepo) DeleteMessage(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET is_deleted=true WHERE id=$1`, id)
	return err
}

func (r *DialogRepo) PinMessage(ctx context.Context, id int64, pinned bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET is_pinned=$1 WHERE id=$2`, pinned, id)
	return err
}

func (r *DialogRepo) ClearHistory(ctx context.Context, dialogID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE messages SET is_deleted=true WHERE dialog_id=$1`, dialogID)
	return err
}

func (r *DialogRepo) GetUserDialogs(ctx context.Context, userID int64) ([]*models.Dialog, error) {
	var dialogs []*models.Dialog
	err := r.db.SelectContext(ctx, &dialogs,
		`SELECT * FROM dialogs WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	return dialogs, err
}
