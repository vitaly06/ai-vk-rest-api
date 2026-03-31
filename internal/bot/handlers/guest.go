package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SevereCloud/vksdk/v3/object"
	"github.com/vitaly06/ai-vk-bot/internal/bot/keyboards"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	inviteSvc "github.com/vitaly06/ai-vk-bot/internal/services/invite"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"
)

// GuestHandler — обработчик для незарегистрированных пользователей
type GuestHandler struct {
	base      *baseHandler
	userSvc   *userSvc.Service
	inviteSvc *inviteSvc.Service
	handlers  *Handlers
}

func (h *GuestHandler) Handle(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	token := extractInviteToken(msg)

	if token != "" {
		inv, err := h.inviteSvc.Validate(ctx, token, u.VKID)
		if err == nil && inv != nil {
			if err := h.userSvc.ActivateWithInvite(ctx, u.VKID, inv.ID); err != nil {
				h.base.send(ctx, u.VKID,
					"❌ Не удалось активировать доступ по ссылке. Попробуйте позже.",
					keyboards.Empty())
				return
			}
			h.base.send(ctx, u.VKID,
				"✅ Ссылка принята! Добро пожаловать в бот.",
				keyboards.Empty())
			return
		}
		h.base.send(ctx, u.VKID,
			"❌ Ссылка недействительна или уже использована.",
			keyboards.Empty())
		return
	}

	if cmd == "request_access" {
		h.sendAccessRequestForm(ctx, u)
		return
	}

	if cmd == "submit_request" {
		h.handleSubmitRequest(ctx, u, text)
		return
	}

	// Если пользователь уже подал заявку — не даём подавать повторно
	if u.State == models.StateAwaitApproval {
		h.base.send(ctx, u.VKID,
			"⏳ Ваша заявка уже на рассмотрении. Ожидайте ответа администратора.",
			keyboards.Empty())
		return
	}

	h.base.send(ctx, u.VKID,
		"👋 Для доступа к боту требуется ссылка-приглашение.\n\n"+
			"Если у вас нет ссылки — подайте заявку на вступление.",
		keyboards.RequestAccess())
}

func (h *GuestHandler) sendAccessRequestForm(ctx context.Context, u *models.User) {
	h.userSvc.UpdateState(ctx, u.VKID, models.StateAwaitRequestText)
	h.base.send(ctx, u.VKID,
		"📝 Напишите, зачем вам нужен доступ к боту, и отправьте сообщение.\n\n"+
			"Администратор рассмотрит вашу заявку.",
		&keyboards.Keyboard{
			OneTime: true,
			Buttons: [][]keyboards.Button{
				{keyboards.MakeBtn("❌ Отмена", "secondary", `{"cmd":"cancel_request"}`)},
			},
		})
}

func (h *GuestHandler) handleSubmitRequest(ctx context.Context, u *models.User, text string) {
	if text == "" {
		h.base.send(ctx, u.VKID, "Пожалуйста, напишите причину запроса.", keyboards.Empty())
		return
	}
	req, err := h.userSvc.CreateAccessRequest(ctx, u.VKID, text)
	if err != nil {
		h.base.send(ctx, u.VKID, "❌ Ошибка отправки заявки. Попробуйте позже.", keyboards.Empty())
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.StateAwaitApproval)
	h.base.send(ctx, u.VKID,
		"📩 Заявка отправлена! Ожидайте ответа администратора.",
		keyboards.Empty())

	// Уведомляем всех админов/модераторов
	notifyText := fmt.Sprintf(
		"📬 Новая заявка на вступление\n\n"+
			"👤 %s\n"+
			"💬 Сообщение: %s",
		u.DisplayName(), text,
	)
	h.handlers.NotifyAdmins(ctx, notifyText, keyboards.AccessRequestActions(req.ID, u.VKID, u.DisplayName()))
}

// extractInviteToken извлекает токен из payload сообщения.
// VK передаёт ref через payload при переходе по deep link: {"ref":"<token>"}
func extractInviteToken(msg object.MessagesMessage) string {
	if msg.Payload != "" {
		var payload map[string]string
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err == nil && payload["ref"] != "" {
			return payload["ref"]
		}
	}

	raw, err := json.Marshal(msg)
	if err == nil {
		var meta map[string]any
		if err := json.Unmarshal(raw, &meta); err == nil {
			if ref, ok := meta["ref"].(string); ok && ref != "" {
				return ref
			}
		}
	}

	return parseInviteTokenFromText(msg.Text)
}

func parseInviteTokenFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	parts := strings.Fields(text)
	if len(parts) == 2 && strings.EqualFold(parts[0], "/start") {
		return parts[1]
	}

	return ""
}
