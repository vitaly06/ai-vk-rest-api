package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v3/object"
	"github.com/vitaly06/ai-vk-bot/internal/bot/keyboards"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	inviteSvc "github.com/vitaly06/ai-vk-bot/internal/services/invite"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"
)

// ModeratorHandler — обработчик команд модератора
type ModeratorHandler struct {
	base      *baseHandler
	userSvc   *userSvc.Service
	inviteSvc *inviteSvc.Service
	handlers  *Handlers
}

func (h *ModeratorHandler) Handle(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "mod_invite":
		h.handleCreateInvite(ctx, u)
	case "mod_support":
		h.base.send(ctx, u.VKID, "📋 Открытые обращения:", keyboards.ModeratorMenu())
	case "mod_check_user":
		h.base.send(ctx, u.VKID, "Введите VK ID пользователя для проверки:\n/checkuser <vk_id>", keyboards.ModeratorMenu())
	case "approve_request":
		h.handleAccessDecision(ctx, u, msg, true)
	case "reject_request":
		h.handleAccessDecision(ctx, u, msg, false)
	default:
		h.handleTextCommand(ctx, u, text)
	}
}

func (h *ModeratorHandler) handleCreateInvite(ctx context.Context, u *models.User) {
	link, err := h.inviteSvc.Create(ctx, u.VKID, 1)
	if err != nil {
		h.base.send(ctx, u.VKID, "❌ Ошибка: "+err.Error(), keyboards.ModeratorMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("🔗 Ссылка-приглашение:\n%s\n\nДействует 72 часа.", link.URL),
		keyboards.ModeratorMenu())
}

func (h *ModeratorHandler) handleAccessDecision(ctx context.Context, actor *models.User, msg object.MessagesMessage, approve bool) {
	var payload struct {
		ReqID int64 `json:"req_id"`
		VKID  int64 `json:"vk_id"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ReqID == 0 {
		h.base.send(ctx, actor.VKID, "❌ Ошибка: не удалось прочитать данные заявки.", keyboards.ModeratorMenu())
		return
	}

	if approve {
		applicantVKID, err := h.userSvc.ApproveAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "❌ Ошибка одобрения: "+err.Error(), keyboards.ModeratorMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("✅ Заявка #%d одобрена.", payload.ReqID),
			keyboards.ModeratorMenu())
		h.base.send(ctx, applicantVKID,
			"🎉 Ваша заявка одобрена! Напишите что-нибудь чтобы начать.",
			keyboards.Empty())
	} else {
		applicantVKID, err := h.userSvc.RejectAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "❌ Ошибка отклонения: "+err.Error(), keyboards.ModeratorMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("❌ Заявка #%d отклонена.", payload.ReqID),
			keyboards.ModeratorMenu())
		if applicantVKID > 0 {
			h.base.send(ctx, applicantVKID,
				"😔 Ваша заявка на вступление отклонена.",
				keyboards.Empty())
		}
	}
}

func (h *ModeratorHandler) handleTextCommand(ctx context.Context, u *models.User, text string) {
	switch {
	case strings.HasPrefix(text, "/checkuser "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		target, err := h.userSvc.GetByVKID(ctx, targetID)
		if err != nil || target == nil {
			h.base.send(ctx, u.VKID, "Пользователь не найден.", keyboards.ModeratorMenu())
			return
		}
		banInfo := "нет"
		if target.BannedUntil != nil {
			banInfo = target.BannedUntil.Format("02.01.2006 15:04")
		}
		info := fmt.Sprintf(
			"👤 [id%d|%s %s]\nРоль: %s\nСтатус: %s\nЗапросов: %d (лимит: %d)\nОхлаждение до: %s\nРегистрация: %s",
			target.VKID, target.FirstName, target.LastName,
			target.Role, target.Status,
			target.RequestCount, target.RequestLimit,
			banInfo,
			target.CreatedAt.Format("02.01.2006"),
		)
		h.base.send(ctx, u.VKID, info, keyboards.ModeratorMenu())

	case strings.HasPrefix(text, "/restrict "):
		// /restrict <vk_id> <minutes>
		parts := strings.Fields(text)
		if len(parts) < 3 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		mins, _ := strconv.Atoi(parts[2])
		until := time.Now().Add(time.Duration(mins) * time.Minute)
		h.userSvc.Ban(ctx, u.VKID, targetID, &until)
		h.base.send(ctx, u.VKID,
			fmt.Sprintf("❄️ Пользователь %d ограничен до %s.", targetID, until.Format("02.01 15:04")),
			keyboards.ModeratorMenu())

	case strings.HasPrefix(text, "/unban "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.Unban(ctx, targetID)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Ограничение снято с %d.", targetID), keyboards.ModeratorMenu())

	default:
		h.base.send(ctx, u.VKID, "👮 Панель модератора", keyboards.ModeratorMenu())
	}
}
