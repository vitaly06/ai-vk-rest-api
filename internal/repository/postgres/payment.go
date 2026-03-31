package postgres

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type PaymentRepo struct {
	db *sqlx.DB
}

func NewPaymentRepo(db *sqlx.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

func (r *PaymentRepo) CreatePayment(ctx context.Context, p *models.Payment) (*models.Payment, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO payments (user_id, external_id, amount, currency, method, status, description, confirmation_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`,
		p.UserID, p.ExternalID, p.Amount, p.Currency,
		p.Method, p.Status, p.Description, p.ConfirmationURL,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *PaymentRepo) GetPayment(ctx context.Context, id int64) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.db.GetContext(ctx, p, `SELECT * FROM payments WHERE id=$1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PaymentRepo) GetByExternalID(ctx context.Context, externalID string) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.db.GetContext(ctx, p, `SELECT * FROM payments WHERE external_id=$1`, externalID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PaymentRepo) UpdateStatus(ctx context.Context, id int64, status models.PaymentStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE payments SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *PaymentRepo) ListByUser(ctx context.Context, userID int64) ([]*models.Payment, error) {
	var payments []*models.Payment
	err := r.db.SelectContext(ctx, &payments,
		`SELECT * FROM payments WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	return payments, err
}

func (r *PaymentRepo) ListProducts(ctx context.Context) ([]*models.Product, error) {
	var products []*models.Product
	err := r.db.SelectContext(ctx, &products,
		`SELECT * FROM products WHERE is_active=true ORDER BY price ASC`)
	return products, err
}

func (r *PaymentRepo) GetProduct(ctx context.Context, id int64) (*models.Product, error) {
	p := &models.Product{}
	err := r.db.GetContext(ctx, p, `SELECT * FROM products WHERE id=$1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PaymentRepo) CreateProduct(ctx context.Context, p *models.Product) (*models.Product, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO products (name, description, price, currency, is_active)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, created_at`,
		p.Name, p.Description, p.Price, p.Currency, p.IsActive,
	).Scan(&p.ID, &p.CreatedAt)
	return p, err
}

func (r *PaymentRepo) UpdateProduct(ctx context.Context, p *models.Product) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE products SET name=$1, description=$2, price=$3, currency=$4, is_active=$5
		WHERE id=$6`,
		p.Name, p.Description, p.Price, p.Currency, p.IsActive, p.ID)
	return err
}
