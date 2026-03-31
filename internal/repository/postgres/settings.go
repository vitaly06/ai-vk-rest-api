package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type SettingsRepo struct {
	db *sqlx.DB
}

func NewSettingsRepo(db *sqlx.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.GetContext(ctx, &value, `SELECT value FROM bot_settings WHERE key=$1`, key)
	return value, err
}

func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO bot_settings (key, value, updated_at)
		VALUES ($1,$2,NOW())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`,
		key, value)
	return err
}

func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM bot_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

func (r *SettingsRepo) WriteAuditLog(ctx context.Context, log *models.AuditLog) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO audit_logs (actor_id, target_id, action, details)
		VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		log.ActorID, log.TargetID, log.Action, log.Details,
	).Scan(&log.ID, &log.CreatedAt)
	return err
}

func (r *SettingsRepo) GetAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog
	err := r.db.SelectContext(ctx, &logs,
		`SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	return logs, err
}
