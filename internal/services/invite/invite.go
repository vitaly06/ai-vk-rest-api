package invite

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
)

const vkDeepLinkBase = "https://vk.me/%s?ref=%s"

// Service — управление ссылками-приглашениями
type Service struct {
	repo        repository.InviteRepository
	communityID string // slug сообщества VK, например "my_community"
	ttlHours    int
}

func New(repo repository.InviteRepository, communityID string, ttlHours int) *Service {
	return &Service{
		repo:        repo,
		communityID: communityID,
		ttlHours:    ttlHours,
	}
}

// Create генерирует новую ссылку. maxUses=1 — одноразовая, 0 — многоразовая до TTL.
func (s *Service) Create(ctx context.Context, createdByVKID int64, maxUses int) (*models.InviteLink, error) {
	token := uuid.New().String()
	expires := time.Now().Add(time.Duration(s.ttlHours) * time.Hour)

	inv, err := s.repo.Create(ctx, &models.Invite{
		Token:       token,
		CreatedByID: createdByVKID,
		MaxUses:     maxUses,
		ExpiresAt:   &expires,
	})
	if err != nil {
		return nil, fmt.Errorf("invite create: %w", err)
	}

	return &models.InviteLink{
		Invite: inv,
		URL:    fmt.Sprintf(vkDeepLinkBase, s.communityID, token),
	}, nil
}

// Validate проверяет токен и засчитывает использование.
// Возвращает nil если ссылка недействительна.
func (s *Service) Validate(ctx context.Context, token string, userVKID int64) (*models.Invite, error) {
	inv, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invite get: %w", err)
	}
	if inv == nil || !inv.IsValid() {
		return nil, nil
	}
	if err := s.repo.Use(ctx, token, userVKID); err != nil {
		return nil, fmt.Errorf("invite use: %w", err)
	}
	return inv, nil
}

func (s *Service) ListAll(ctx context.Context) ([]*models.Invite, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) ListByCreator(ctx context.Context, creatorID int64) ([]*models.Invite, error) {
	return s.repo.ListByCreator(ctx, creatorID)
}
