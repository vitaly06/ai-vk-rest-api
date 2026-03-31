package models

import "time"

// PaymentStatus — статус платежа
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCanceled  PaymentStatus = "canceled"
)

// PaymentMethod — способ оплаты
type PaymentMethod string

const (
	PaymentMethodSBP    PaymentMethod = "sbp"
	PaymentMethodCard   PaymentMethod = "bank_card"
	PaymentMethodWallet PaymentMethod = "wallet" // внутренний кошелёк
	PaymentMethodCrypto PaymentMethod = "crypto"
)

// Payment — запись о транзакции
type Payment struct {
	ID              int64         `db:"id"`
	UserID          int64         `db:"user_id"`
	ExternalID      string        `db:"external_id"` // ID в платёжной системе
	Amount          float64       `db:"amount"`
	Currency        string        `db:"currency"` // RUB | USDT
	Method          PaymentMethod `db:"method"`
	Status          PaymentStatus `db:"status"`
	Description     string        `db:"description"`
	ConfirmationURL string        `db:"confirmation_url"`
	CreatedAt       time.Time     `db:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"`
}

// Product — услуга/товар в каталоге
type Product struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Price       float64   `db:"price"`
	Currency    string    `db:"currency"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
}

// CartItem — позиция в корзине (хранится в Redis)
type CartItem struct {
	ProductID int64   `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}
