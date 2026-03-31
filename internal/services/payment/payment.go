package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/vitaly06/ai-vk-bot/internal/config"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
)

const yooKassaURL = "https://api.yookassa.ru/v3/payments"

// Service — управление платежами через YooKassa
type Service struct {
	cfg      config.PaymentConfig
	repo     repository.PaymentRepository
	userRepo repository.UserRepository
	client   *http.Client
}

func New(cfg config.PaymentConfig, repo repository.PaymentRepository, userRepo repository.UserRepository) *Service {
	return &Service{
		cfg:      cfg,
		repo:     repo,
		userRepo: userRepo,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type yooRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Description string `json:"description"`
	Capture     bool   `json:"capture"`
}

type yooResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

// CreatePayment создаёт платёж в YooKassa и сохраняет запись в БД
func (s *Service) CreatePayment(ctx context.Context, userID int64, amount float64, description string) (*models.Payment, error) {
	reqBody := yooRequest{
		Description: description,
		Capture:     true,
	}
	reqBody.Amount.Value = fmt.Sprintf("%.2f", amount)
	reqBody.Amount.Currency = "RUB"
	reqBody.Confirmation.Type = "redirect"
	reqBody.Confirmation.ReturnURL = s.cfg.ReturnURL

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, yooKassaURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("payment: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", uuid.New().String())
	req.SetBasicAuth(s.cfg.YooKassaShopID, s.cfg.YooKassaSecretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payment: do request: %w", err)
	}
	defer resp.Body.Close()

	var yr yooResponse
	if err := json.NewDecoder(resp.Body).Decode(&yr); err != nil {
		return nil, fmt.Errorf("payment: decode: %w", err)
	}

	p := &models.Payment{
		UserID:          userID,
		ExternalID:      yr.ID,
		Amount:          amount,
		Currency:        "RUB",
		Method:          models.PaymentMethodCard,
		Status:          models.PaymentStatusPending,
		Description:     description,
		ConfirmationURL: yr.Confirmation.ConfirmationURL,
	}
	return s.repo.CreatePayment(ctx, p)
}

// HandleWebhook обрабатывает уведомление от YooKassa
func (s *Service) HandleWebhook(ctx context.Context, externalID, status string) error {
	p, err := s.repo.GetByExternalID(ctx, externalID)
	if err != nil || p == nil {
		return fmt.Errorf("payment: not found %s", externalID)
	}
	var newStatus models.PaymentStatus
	switch status {
	case "succeeded":
		newStatus = models.PaymentStatusSucceeded
		// Пополняем баланс пользователя
		if err := s.userRepo.UpdateBalance(ctx, p.UserID, p.Amount); err != nil {
			return err
		}
	case "canceled":
		newStatus = models.PaymentStatusCanceled
	default:
		return nil
	}
	return s.repo.UpdateStatus(ctx, p.ID, newStatus)
}

func (s *Service) ListProducts(ctx context.Context) ([]*models.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id int64) (*models.Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) ListByUser(ctx context.Context, userID int64) ([]*models.Payment, error) {
	return s.repo.ListByUser(ctx, userID)
}
