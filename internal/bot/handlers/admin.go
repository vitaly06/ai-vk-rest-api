package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v3/object"
	"github.com/vitaly06/ai-vk-bot/internal/bot/keyboards"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
	aiSvc "github.com/vitaly06/ai-vk-bot/internal/services/ai"
	inviteSvc "github.com/vitaly06/ai-vk-bot/internal/services/invite"
	monSvc "github.com/vitaly06/ai-vk-bot/internal/services/monitoring"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"
)

// AdminHandler — обработчик команд администратора
type AdminHandler struct {
	base         *baseHandler
	userSvc      *userSvc.Service
	inviteSvc    *inviteSvc.Service
	aiSvc        *aiSvc.Service
	dialogRepo   repository.DialogRepository
	settingsRepo repository.SettingsRepository
	mon          *monSvc.Service
	handlers     *Handlers
}

func (h *AdminHandler) Handle(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "admin_invites":
		h.handleInviteMenu(ctx, u)
	case "admin_users":
		h.handleUsersPage(ctx, u, 0)
	case "admin_users_page":
		var p struct {
			Offset int `json:"offset"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.handleUsersPage(ctx, u, p.Offset)
	case "admin_user_detail":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.handleUserDetail(ctx, u, p.VKID)
	case "admin_ban":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.Ban(ctx, u.VKID, p.VKID, nil)
		h.base.send(ctx, u.VKID, fmt.Sprintf("🚫 Пользователь [id%d] заблокирован.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "ban", "")
	case "admin_cool":
		var p struct {
			VKID int64 `json:"vk_id"`
			Mins int   `json:"mins"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		until := time.Now().Add(time.Duration(p.Mins) * time.Minute)
		h.userSvc.Ban(ctx, u.VKID, p.VKID, &until)
		h.base.send(ctx, u.VKID,
			fmt.Sprintf("❄️ [id%d] в охлаждении на %d мин. (до %s).", p.VKID, p.Mins, until.Format("15:04")),
			keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "cooldown", fmt.Sprintf("%dmin", p.Mins))
	case "admin_unban":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.Unban(ctx, p.VKID)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Ограничение снято с [id%d].", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "unban", "")
	case "admin_set_mod":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("👮 [id%d] назначен модератором.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_mod", "")
	case "admin_set_user":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("👤 [id%d] разжалован до пользователя.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_user", "")
	case "admin_set_limit":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.base.send(ctx, u.VKID,
			fmt.Sprintf("Введите новый лимит запросов для [id%d]:\n/setlimit %d <число>", p.VKID, p.VKID),
			keyboards.AdminMenu())
	case "admin_metrics":
		h.handleMetrics(ctx, u)
	case "admin_settings":
		h.handleSettingsMenu(ctx, u)
	case "admin_mods":
		h.handleModsMenu(ctx, u)
	case "approve_request":
		h.handleAccessDecision(ctx, u, msg, true)
	case "reject_request":
		h.handleAccessDecision(ctx, u, msg, false)
	case "main_chat":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
		h.base.send(ctx, u.VKID, "💬 Режим диалога с AI. Пишите вопрос.", keyboards.AdminChatMenu())
	case "support":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID, "🛠 Чат поддержки.", keyboards.BackOnly())
	case "admin_panel":
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "👑 Панель администратора", keyboards.AdminMenu())
	default:
		// Если админ в режиме диалога — отправляем в AI
		if u.State == models.StateMainChat || u.State == models.StateSupport {
			h.handleAIChat(ctx, u, msg, cmd, text)
		} else {
			h.handleTextCommand(ctx, u, text)
		}
	}
}

func (h *AdminHandler) handleInviteMenu(ctx context.Context, u *models.User) {
	link, err := h.inviteSvc.Create(ctx, u.VKID, 1) // одноразовая
	if err != nil {
		h.base.send(ctx, u.VKID, "❌ Ошибка создания ссылки: "+err.Error(), keyboards.AdminMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("✅ Новая ссылка-приглашение (одноразовая):\n%s\n\nДействует 72 часа.", link.URL),
		keyboards.AdminMenu())
}

func (h *AdminHandler) handleUsersPage(ctx context.Context, u *models.User, offset int) {
	allUsers, err := h.userSvc.ListAll(ctx)
	if err != nil {
		h.base.send(ctx, u.VKID, "❌ Ошибка получения пользователей", keyboards.AdminMenu())
		return
	}
	if len(allUsers) == 0 {
		h.base.send(ctx, u.VKID, "Пользователей нет.", keyboards.AdminMenu())
		return
	}
	end := offset + 8
	if end > len(allUsers) {
		end = len(allUsers)
	}
	page := allUsers[offset:end]

	btns := make([]keyboards.UserButton, 0, len(page))
	for _, usr := range page {
		name := strings.TrimSpace(usr.FirstName + " " + usr.LastName)
		if name == "" {
			name = fmt.Sprintf("id%d", usr.VKID)
		}
		roleIcon := "👤"
		if usr.Role == models.RoleModerator {
			roleIcon = "👮"
		} else if usr.Role == models.RoleAdmin {
			roleIcon = "👑"
		}
		statusIcon := ""
		if usr.Status == models.StatusBanned || usr.Status == models.StatusRestricted {
			statusIcon = " 🚫"
		}
		btns = append(btns, keyboards.UserButton{
			VKID: usr.VKID,
			Name: roleIcon + " " + name + statusIcon,
		})
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("👥 Пользователи (%d–%d из %d)\n\nНажмите на пользователя:", offset+1, end, len(allUsers)),
		keyboards.UserListInline(btns, offset, len(allUsers)))
}

func (h *AdminHandler) handleUserDetail(ctx context.Context, admin *models.User, targetVKID int64) {
	target, err := h.userSvc.GetByVKID(ctx, targetVKID)
	if err != nil || target == nil {
		h.base.send(ctx, admin.VKID, "❌ Пользователь не найден.", keyboards.AdminMenu())
		return
	}
	statusEmoji := "🟢"
	switch target.Status {
	case models.StatusBanned:
		statusEmoji = "🔴"
	case models.StatusRestricted:
		statusEmoji = "🟡"
	case models.StatusPending:
		statusEmoji = "⚪"
	}
	banInfo := "—"
	if target.BannedUntil != nil {
		banInfo = target.BannedUntil.Format("02.01.2006 15:04")
	}
	limitStr := "∞"
	if target.RequestLimit > 0 {
		limitStr = fmt.Sprintf("%d", target.RequestLimit)
	}
	text := fmt.Sprintf(
		"👤 [id%d|%s %s]\n\n"+
			"Роль: %s\n"+
			"%s Статус: %s\n"+
			"💬 Запросов: %d / %s\n"+
			"💰 Баланс: %.2f ₽\n"+
			"🕐 Ограничен до: %s\n"+
			"📅 Регистрация: %s",
		target.VKID, target.FirstName, target.LastName,
		target.Role,
		statusEmoji, target.Status,
		target.RequestCount, limitStr,
		target.Balance,
		banInfo,
		target.CreatedAt.Format("02.01.2006"),
	)
	h.base.send(ctx, admin.VKID, text, keyboards.UserActionsInline(targetVKID))
}

func (h *AdminHandler) handleMetrics(ctx context.Context, u *models.User) {
	m := h.mon.GetMetrics(ctx)
	text := fmt.Sprintf(
		"📊 Мониторинг\n\n"+
			"👤 Активных: %d\n"+
			"🤖 AI запросов сегодня: %d\n"+
			"❌ Ошибок сегодня: %d\n"+
			"💾 Память: %.1f МБ\n"+
			"⏱ Uptime: %s",
		m.ActiveUsers,
		m.AICallsToday,
		m.ErrorsToday,
		m.MemoryUsageMB,
		fmtDuration(time.Duration(m.UptimeSeconds)*time.Second),
	)
	h.base.send(ctx, u.VKID, text, keyboards.AdminMenu())
}

func (h *AdminHandler) handleSettingsMenu(ctx context.Context, u *models.User) {
	settings, _ := h.settingsRepo.GetAll(ctx)
	welcome := settings[models.SettingWelcomeMessage]
	if welcome == "" {
		welcome = "(не задано)"
	}
	h.base.send(ctx, u.VKID,
		"⚙️ Настройки бота\n\nЧтобы изменить приветственное сообщение, напишите:\n/setwelcome <текст>\n\nТекущее:\n"+welcome,
		keyboards.AdminMenu())
}

func (h *AdminHandler) handleModsMenu(ctx context.Context, u *models.User) {
	mods, _ := h.userSvc.ListAll(ctx)
	var sb strings.Builder
	sb.WriteString("👮 Модераторы:\n")
	count := 0
	for _, m := range mods {
		if m.Role == models.RoleModerator {
			sb.WriteString(fmt.Sprintf("• [id%d|%s %s]\n", m.VKID, m.FirstName, m.LastName))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("Нет модераторов\n")
	}
	sb.WriteString("\nКоманды:\n/addmod <vk_id> — добавить модератора\n/delmod <vk_id> — удалить")
	h.base.send(ctx, u.VKID, sb.String(), keyboards.AdminMenu())
}

func (h *AdminHandler) handleTextCommand(ctx context.Context, u *models.User, text string) {
	switch {
	case strings.HasPrefix(text, "/setwelcome "):
		msg := strings.TrimPrefix(text, "/setwelcome ")
		h.settingsRepo.Set(ctx, models.SettingWelcomeMessage, msg)
		h.base.send(ctx, u.VKID, "✅ Приветственное сообщение обновлено.", keyboards.AdminMenu())
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: u.VKID, Action: "set_welcome", Details: msg,
		})

	case strings.HasPrefix(text, "/ban "):
		h.handleBanCommand(ctx, u, text, false)

	case strings.HasPrefix(text, "/cool "):
		h.handleBanCommand(ctx, u, text, true)

	case strings.HasPrefix(text, "/unban "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.Unban(ctx, targetID)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Пользователь %d разблокирован.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/addmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Пользователь %d назначен модератором.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/delmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Модератор %d понижен до пользователя.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/setlimit "):
		// /setlimit <vk_id> <count>
		parts := strings.Fields(text)
		if len(parts) < 3 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		limit, _ := strconv.Atoi(parts[2])
		h.userSvc.SetRequestLimit(ctx, targetID, limit)
		h.base.send(ctx, u.VKID, fmt.Sprintf("✅ Лимит %d запросов установлен для %d.", limit, targetID), keyboards.AdminMenu())

	default:
		// Показываем меню администратора
		h.base.send(ctx, u.VKID, "👑 Панель администратора", keyboards.AdminMenu())
	}
}

func (h *AdminHandler) handleAccessDecision(ctx context.Context, actor *models.User, msg object.MessagesMessage, approve bool) {
	// Читаем req_id и vk_id из payload кнопки
	var payload struct {
		ReqID int64 `json:"req_id"`
		VKID  int64 `json:"vk_id"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ReqID == 0 {
		h.base.send(ctx, actor.VKID, "❌ Ошибка: не удалось прочитать данные заявки.", keyboards.AdminMenu())
		return
	}

	if approve {
		applicantVKID, err := h.userSvc.ApproveAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "❌ Ошибка одобрения: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("✅ Заявка #%d одобрена. Пользователь [id%d|активирован].", payload.ReqID, applicantVKID),
			keyboards.AdminMenu())
		// Уведомляем заявителя
		h.base.send(ctx, applicantVKID,
			"🎉 Ваша заявка одобрена! Напишите что-нибудь чтобы начать.",
			keyboards.Empty())
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "approve_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	} else {
		applicantVKID, err := h.userSvc.RejectAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "❌ Ошибка отклонения: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("❌ Заявка #%d отклонена.", payload.ReqID),
			keyboards.AdminMenu())
		// Уведомляем заявителя
		if applicantVKID > 0 {
			h.base.send(ctx, applicantVKID,
				"😔 Ваша заявка на вступление отклонена.",
				keyboards.Empty())
		}
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "reject_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	}
}

func (h *AdminHandler) handleBanCommand(ctx context.Context, u *models.User, text string, isCooldown bool) {
	// /ban <vk_id> или /cool <vk_id> <minutes>
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return
	}
	targetID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}

	var until *time.Time
	if isCooldown && len(parts) >= 3 {
		mins, _ := strconv.Atoi(parts[2])
		t := time.Now().Add(time.Duration(mins) * time.Minute)
		until = &t
	}

	h.userSvc.Ban(ctx, u.VKID, targetID, until)
	msg := fmt.Sprintf("✅ Пользователь %d заблокирован.", targetID)
	if until != nil {
		msg = fmt.Sprintf("❄️ Пользователь %d в охлаждении до %s.", targetID, until.Format("02.01 15:04"))
	}
	h.base.send(ctx, u.VKID, msg, keyboards.AdminMenu())
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID: u.VKID, TargetID: &targetID, Action: "ban", Details: text,
	})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dч %dм", h, m)
}

func (h *AdminHandler) logAudit(ctx context.Context, actorID, targetID int64, action, details string) {
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID:  actorID,
		TargetID: &targetID,
		Action:   action,
		Details:  details,
	})
}

// handleAIChat — AI-диалог для администратора (аналог user.go, но без лимитов)
func (h *AdminHandler) handleAIChat(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if cmd == "back" || cmd == "admin_panel" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "👑 Панель администратора", keyboards.AdminMenu())
		return
	}
	if u.State == models.StateSupport {
		if cmd == "back" {
			h.userSvc.UpdateState(ctx, u.VKID, "")
			h.base.send(ctx, u.VKID, "👑 Панель администратора", keyboards.AdminMenu())
			return
		}
		dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
		h.dialogRepo.SaveMessage(ctx, &models.Message{
			DialogID: dialog.ID, UserID: u.ID,
			Role: models.MessageRoleUser, Type: models.MessageTypeText, Content: text,
		})
		h.base.send(ctx, u.VKID, "✅ Сообщение сохранено в поддержке.", keyboards.BackOnly())
		return
	}

	if strings.TrimSpace(text) == "" {
		return
	}

	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogMain)
	if err != nil {
		slog.Error("admin get dialog", "err", err)
		return
	}

	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID, UserID: u.ID,
		Role: models.MessageRoleUser, Type: models.MessageTypeText, Content: text,
	})

	history, _ := h.dialogRepo.GetHistory(ctx, dialog.ID, 20)
	aiMessages := make([]models.AIMessage, 0, len(history))
	for _, m := range history {
		role := "user"
		if m.Role == models.MessageRoleAssistant {
			role = "assistant"
		}
		aiMessages = append(aiMessages, models.AIMessage{Role: role, Content: m.Content})
	}

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("admin ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "⚠️ Ошибка AI: "+err.Error(), keyboards.AdminChatMenu())
		return
	}

	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID, UserID: u.ID,
		Role: models.MessageRoleAssistant, Type: models.MessageTypeText, Content: reply,
	})

	h.base.send(ctx, u.VKID, reply, keyboards.AdminChatMenu())
}
