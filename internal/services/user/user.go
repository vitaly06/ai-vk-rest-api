package user

import (
	"context"
	"fmt"
	"time"

	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
	"github.com/vitaly06/ai-vk-bot/internal/repository/redisstore"
)

// Service — управление пользователями
type Service struct {
	repo  repository.UserRepository
	redis *redisstore.Store
}

func New(repo repository.UserRepository, redis *redisstore.Store) *Service {
	return &Service{repo: repo, redis: redis}
}

// GetOrCreate возвращает пользователя или создаёт нового гостя
func (s *Service) GetOrCreate(ctx context.Context, vkID int64, firstName, lastName, username string) (*models.User, error) {
	u, err := s.repo.GetByVKID(ctx, vkID)
	if err != nil {
		return nil, fmt.Errorf("user get: %w", err)
	}
	if u != nil {
		return u, nil
	}
	u, err = s.repo.Create(ctx, &models.User{
		VKID:      vkID,
		FirstName: firstName,
		LastName:  lastName,
		Username:  username,
		Role:      models.RoleGuest,
		Status:    models.StatusPending,
		State:     models.StateWelcome,
	})
	return u, err
}

func (s *Service) SyncProfile(ctx context.Context, vkID int64, firstName, lastName, username string) (*models.User, error) {
	u, err := s.repo.GetByVKID(ctx, vkID)
	if err != nil {
		return nil, fmt.Errorf("user get: %w", err)
	}
	if u == nil {
		return s.GetOrCreate(ctx, vkID, firstName, lastName, username)
	}

	if u.FirstName == firstName && u.LastName == lastName && u.Username == username {
		return u, nil
	}

	u.FirstName = firstName
	u.LastName = lastName
	u.Username = username

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("user update: %w", err)
	}

	return u, nil
}

// ActivateWithInvite переводит пользователя в активное состояние после использования ссылки
func (s *Service) ActivateWithInvite(ctx context.Context, vkID int64, inviteID int64) error {
	u, err := s.repo.GetByVKID(ctx, vkID)
	if err != nil || u == nil {
		return fmt.Errorf("user not found: %w", err)
	}
	u.Role = models.RoleUser
	u.Status = models.StatusActive
	u.State = models.StateWelcome
	u.InviteID = &inviteID
	return s.repo.Update(ctx, u)
}

// CheckLimit проверяет, не превышен ли лимит запросов к AI
func (s *Service) CheckLimit(ctx context.Context, u *models.User) (bool, error) {
	if u.RequestLimit == 0 {
		return true, nil // безлимит
	}
	cnt, err := s.redis.IncrRequestCount(ctx, u.VKID)
	if err != nil {
		return false, err
	}
	return int(cnt) <= u.RequestLimit, nil
}

// IsRestricted проверяет, находится ли пользователь в охлаждении
func (s *Service) IsRestricted(u *models.User) bool {
	if u.Status == models.StatusBanned {
		return true
	}
	if u.Status == models.StatusRestricted && u.BannedUntil != nil {
		if time.Now().Before(*u.BannedUntil) {
			return true
		}
	}
	return false
}

// Ban блокирует пользователя
func (s *Service) Ban(ctx context.Context, actorID, targetVKID int64, until *time.Time) error {
	status := models.StatusBanned
	if until != nil {
		status = models.StatusRestricted
	}
	return s.repo.UpdateStatus(ctx, targetVKID, status, until)
}

// Unban снимает ограничения
func (s *Service) Unban(ctx context.Context, targetVKID int64) error {
	return s.repo.UpdateStatus(ctx, targetVKID, models.StatusActive, nil)
}

// SetRole назначает роль пользователю
func (s *Service) SetRole(ctx context.Context, targetVKID int64, role models.Role) error {
	return s.repo.UpdateRole(ctx, targetVKID, role)
}

// SetRequestLimit задаёт лимит запросов
func (s *Service) SetRequestLimit(ctx context.Context, targetVKID int64, limit int) error {
	return s.repo.SetRequestLimit(ctx, targetVKID, limit)
}

// UpdateState обновляет FSM-состояние пользователя
func (s *Service) UpdateState(ctx context.Context, vkID int64, state models.BotState) error {
	return s.repo.UpdateState(ctx, vkID, state)
}

// SaveConsent сохраняет согласие на обработку данных и рассылки
func (s *Service) SaveConsent(ctx context.Context, vkID int64, dataConsent, mailingConsent bool) error {
	u, err := s.repo.GetByVKID(ctx, vkID)
	if err != nil || u == nil {
		return fmt.Errorf("user not found: %w", err)
	}
	u.ConsentGiven = dataConsent
	u.MailingConsent = mailingConsent
	return s.repo.Update(ctx, u)
}

func (s *Service) GetByVKID(ctx context.Context, vkID int64) (*models.User, error) {
	return s.repo.GetByVKID(ctx, vkID)
}

func (s *Service) ListAll(ctx context.Context) ([]*models.User, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) SaveQAnswer(ctx context.Context, userID int64, question, answer string) error {
	return s.repo.SaveQuestionnaireAnswer(ctx, &models.QuestionnaireAnswer{
		UserID:   userID,
		Question: question,
		Answer:   answer,
	})
}

func (s *Service) CreateAccessRequest(ctx context.Context, vkID int64, msg string) (*models.AccessRequest, error) {
	req := &models.AccessRequest{
		VKID:    vkID,
		Message: msg,
	}
	err := s.repo.CreateAccessRequest(ctx, req)
	return req, err
}

// ApproveAccessRequest одобряет заявку и активирует пользователя
func (s *Service) ApproveAccessRequest(ctx context.Context, requestID int64) (int64, error) {
	reqs, err := s.repo.ListPendingRequests(ctx)
	if err != nil {
		return 0, err
	}
	var req *models.AccessRequest
	for _, r := range reqs {
		if r.ID == requestID {
			req = r
			break
		}
	}
	if req == nil {
		return 0, fmt.Errorf("request %d not found", requestID)
	}
	if err := s.repo.UpdateAccessRequest(ctx, requestID, "approved"); err != nil {
		return 0, err
	}
	// Активируем пользователя
	u, err := s.repo.GetByVKID(ctx, req.VKID)
	if err != nil || u == nil {
		return req.VKID, err
	}
	u.Role = models.RoleUser
	u.Status = models.StatusActive
	u.State = models.StateWelcome
	return req.VKID, s.repo.Update(ctx, u)
}

// RejectAccessRequest отклоняет заявку
func (s *Service) RejectAccessRequest(ctx context.Context, requestID int64) (int64, error) {
	reqs, err := s.repo.ListPendingRequests(ctx)
	if err != nil {
		return 0, err
	}
	var req *models.AccessRequest
	for _, r := range reqs {
		if r.ID == requestID {
			req = r
			break
		}
	}
	if req == nil {
		return 0, fmt.Errorf("request %d not found", requestID)
	}
	err = s.repo.UpdateAccessRequest(ctx, requestID, "rejected")
	if req != nil {
		// Сбрасываем состояние, чтобы пользователь мог подать заявку повторно
		if u, _ := s.repo.GetByVKID(ctx, req.VKID); u != nil {
			u.State = models.StateNone
			s.repo.Update(ctx, u)
		}
		return req.VKID, err
	}
	return 0, err
}

func (s *Service) ListPendingRequests(ctx context.Context) ([]*models.AccessRequest, error) {
	return s.repo.ListPendingRequests(ctx)
}

// GetQAnswers возвращает ответы анкеты пользователя по его DB ID
func (s *Service) GetQAnswers(ctx context.Context, userID int64) ([]*models.QuestionnaireAnswer, error) {
	return s.repo.GetQuestionnaireAnswers(ctx, userID)
}

// IncrRequestCount увеличивает счётчик запросов в БД
func (s *Service) IncrRequestCount(ctx context.Context, vkID int64) error {
	return s.repo.IncrementRequestCount(ctx, vkID)
}
