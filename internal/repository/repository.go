// Package repository определяет интерфейсы репозиториев.
package repository

import (
	"context"
	"time"

	"github.com/vitaly06/ai-vk-bot/internal/models"
)

// UserRepository — операции с пользователями
type UserRepository interface {
	GetByVKID(ctx context.Context, vkID int64) (*models.User, error)
	Create(ctx context.Context, u *models.User) (*models.User, error)
	Update(ctx context.Context, u *models.User) error
	UpdateState(ctx context.Context, vkID int64, state models.BotState) error
	UpdateStatus(ctx context.Context, vkID int64, status models.UserStatus, until *time.Time) error
	UpdateRole(ctx context.Context, vkID int64, role models.Role) error
	UpdateBalance(ctx context.Context, vkID int64, delta float64) error
	IncrementRequestCount(ctx context.Context, vkID int64) error
	SetRequestLimit(ctx context.Context, vkID int64, limit int) error
	ListAll(ctx context.Context) ([]*models.User, error)
	ListByRole(ctx context.Context, role models.Role) ([]*models.User, error)
	SaveQuestionnaireAnswer(ctx context.Context, a *models.QuestionnaireAnswer) error
	GetQuestionnaireAnswers(ctx context.Context, userID int64) ([]*models.QuestionnaireAnswer, error)
	CreateAccessRequest(ctx context.Context, req *models.AccessRequest) error
	ListPendingRequests(ctx context.Context) ([]*models.AccessRequest, error)
	UpdateAccessRequest(ctx context.Context, id int64, status string) error
}

// DialogRepository — операции с диалогами и сообщениями
type DialogRepository interface {
	GetOrCreateDialog(ctx context.Context, userID int64, dtype models.DialogType) (*models.Dialog, error)
	GetDialog(ctx context.Context, id int64) (*models.Dialog, error)
	SaveMessage(ctx context.Context, m *models.Message) (*models.Message, error)
	GetHistory(ctx context.Context, dialogID int64, limit int) ([]*models.Message, error)
	DeleteMessage(ctx context.Context, id int64) error
	PinMessage(ctx context.Context, id int64, pinned bool) error
	ClearHistory(ctx context.Context, dialogID int64) error
	GetUserDialogs(ctx context.Context, userID int64) ([]*models.Dialog, error)
}

// InviteRepository — управление ссылками-приглашениями
type InviteRepository interface {
	Create(ctx context.Context, inv *models.Invite) (*models.Invite, error)
	GetByToken(ctx context.Context, token string) (*models.Invite, error)
	Use(ctx context.Context, token string, usedByVKID int64) error
	ListByCreator(ctx context.Context, creatorID int64) ([]*models.Invite, error)
	ListAll(ctx context.Context) ([]*models.Invite, error)
}

// PaymentRepository — транзакции и продукты
type PaymentRepository interface {
	CreatePayment(ctx context.Context, p *models.Payment) (*models.Payment, error)
	GetPayment(ctx context.Context, id int64) (*models.Payment, error)
	GetByExternalID(ctx context.Context, externalID string) (*models.Payment, error)
	UpdateStatus(ctx context.Context, id int64, status models.PaymentStatus) error
	ListByUser(ctx context.Context, userID int64) ([]*models.Payment, error)
	ListProducts(ctx context.Context) ([]*models.Product, error)
	GetProduct(ctx context.Context, id int64) (*models.Product, error)
	CreateProduct(ctx context.Context, p *models.Product) (*models.Product, error)
	UpdateProduct(ctx context.Context, p *models.Product) error
}

// SettingsRepository — настройки бота и аудит
type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) (map[string]string, error)
	WriteAuditLog(ctx context.Context, log *models.AuditLog) error
	GetAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error)
}
