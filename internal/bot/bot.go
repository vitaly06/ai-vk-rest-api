// Package bot — ядро бота: маршрутизация событий VK в обработчики.
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/api/params"
	"github.com/SevereCloud/vksdk/v3/events"
	longpoll "github.com/SevereCloud/vksdk/v3/longpoll-bot"
	"github.com/vitaly06/ai-vk-bot/internal/bot/handlers"
	"github.com/vitaly06/ai-vk-bot/internal/config"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/services/monitoring"
	"github.com/vitaly06/ai-vk-bot/internal/services/user"
)

// Bot — основная структура бота
type Bot struct {
	cfg        *config.Config
	vk         *api.VK
	lp         *longpoll.LongPoll
	handlers   *handlers.Handlers
	userSvc    *user.Service
	monitoring *monitoring.Service
	adminIDs   map[int64]struct{}
}

// New создаёт и настраивает бота
func New(
	cfg *config.Config,
	h *handlers.Handlers,
	userSvc *user.Service,
	mon *monitoring.Service,
) (*Bot, error) {
	vk := api.NewVK(cfg.VK.Token)

	lp, err := longpoll.NewLongPoll(vk, cfg.VK.GroupID)
	if err != nil {
		return nil, fmt.Errorf("longpoll init: %w", err)
	}

	adminIDs := make(map[int64]struct{}, len(cfg.VK.AdminIDs))
	for _, id := range cfg.VK.AdminIDs {
		adminIDs[int64(id)] = struct{}{}
	}

	b := &Bot{
		cfg:        cfg,
		vk:         vk,
		lp:         lp,
		handlers:   h,
		userSvc:    userSvc,
		monitoring: mon,
		adminIDs:   adminIDs,
	}

	lp.MessageNew(b.onMessageNew)
	return b, nil
}

// Run запускает Long Poll цикл
func (b *Bot) Run() error {
	slog.Info("bot started", "mode", "longpoll")
	return b.lp.Run()
}

// onMessageNew — центральный маршрутизатор входящих сообщений
func (b *Bot) onMessageNew(ctx context.Context, obj events.MessageNewObject) {
	msg := obj.Message
	vkID := int64(msg.FromID)
	text := msg.Text

	slog.Info("message received", "vk_id", vkID, "text", text, "payload", msg.Payload)

	// Определяем payload (нажатие кнопки)
	var payload map[string]string
	if msg.Payload != "" {
		json.Unmarshal([]byte(msg.Payload), &payload)
	}

	// Получаем или создаём пользователя
	u, err := b.userSvc.GetOrCreate(ctx, vkID, "", "", "")
	if err != nil {
		slog.Error("get user", "err", err)
		b.monitoring.RecordError()
		send(b.vk, vkID, "⚠️ Внутренняя ошибка. Попробуйте позже.")
		return
	}
	if vkID > 0 {
		if synced, err := b.syncUserProfile(ctx, vkID); err != nil {
			slog.Warn("sync user profile", "vk_id", vkID, "err", err)
		} else if synced != nil {
			u = synced
		}
	}
	slog.Info("user loaded", "vk_id", vkID, "role", u.Role, "status", u.Status, "state", u.State)

	// Назначаем роль Admin если vkID в списке
	if _, isAdmin := b.adminIDs[vkID]; isAdmin && u.Role != models.RoleAdmin {
		b.userSvc.SetRole(ctx, vkID, models.RoleAdmin)
		b.userSvc.Unban(ctx, vkID) // также переводим в статус active
		u.Role = models.RoleAdmin
		u.Status = models.StatusActive
	}

	// Проверка ограничений
	if u.Role == models.RoleUser && b.userSvc.IsRestricted(u) {
		b.handlers.SendRestricted(ctx, vkID, u)
		return
	}

	// Маршрутизация по роли и команде
	cmd := ""
	if payload != nil {
		cmd = payload["cmd"]
	}

	switch {
	// Администратор
	case u.Role == models.RoleAdmin:
		b.handlers.Admin.Handle(ctx, u, msg, cmd, text)

	// Модератор
	case u.Role == models.RoleModerator:
		b.handlers.Moderator.Handle(ctx, u, msg, cmd, text)

	// Обычный пользователь
	case u.Role == models.RoleUser:
		b.handlers.User.Handle(ctx, u, msg, cmd, text)

	// Гость ожидает ввода текста заявки — проверяем ДО дефолта
	case u.State == models.StateAwaitRequestText:
		if cmd == "cancel_request" {
			b.userSvc.UpdateState(ctx, vkID, models.StateNone)
			b.handlers.Guest.Handle(ctx, u, msg, "", "")
		} else {
			b.handlers.Guest.Handle(ctx, u, msg, "submit_request", text)
		}

	// Гость / ожидает одобрения
	default:
		slog.Info("routing to guest", "vk_id", vkID, "role", u.Role)
		b.handlers.Guest.Handle(ctx, u, msg, cmd, text)
	}
}

func (b *Bot) syncUserProfile(ctx context.Context, vkID int64) (*models.User, error) {
	users, err := b.vk.UsersGet(api.Params{
		"user_ids": []string{strconv.FormatInt(vkID, 10)},
	})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}

	username := users[0].ScreenName
	if username == "" {
		username = users[0].Domain
	}

	return b.userSvc.SyncProfile(ctx, vkID, users[0].FirstName, users[0].LastName, username)
}

// send — прямая отправка сообщения (для экстренных уведомлений)
func send(vk *api.VK, vkID int64, text string) {
	b := params.NewMessagesSendBuilder()
	b.UserID(int(vkID))
	b.Message(text)
	b.RandomID(0)
	_, err := vk.MessagesSend(b.Params)
	if err != nil {
		slog.Error("emergency send failed", "vk_id", vkID, "err", err)
	}
}
