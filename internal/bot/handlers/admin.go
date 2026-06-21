package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
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

// AdminHandler РІР‚вЂќ Р С•Р В±РЎР‚Р В°Р В±Р С•РЎвЂљРЎвЂЎР С‘Р С” Р С”Р С•Р СР В°Р Р…Р Т‘ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°
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

const adminEditStatePrefix = "admin_edit:"
const adminFAQAddState = "admin_faq_add"
const adminFAQEditStatePrefix = "admin_faq_edit:"
const adminQuestionAddState = "admin_q_add"
const adminQuestionEditStatePrefix = "admin_q_edit:"

func (h *AdminHandler) Handle(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if (u.State == models.BotState(adminFAQAddState) || u.State == models.BotState(adminQuestionAddState)) && cmd == "back" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.handleSettingsMenu(ctx, u)
		return
	}
	if (strings.HasPrefix(string(u.State), adminFAQEditStatePrefix) || strings.HasPrefix(string(u.State), adminQuestionEditStatePrefix)) && cmd == "back" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.handleSettingsMenu(ctx, u)
		return
	}
	if strings.HasPrefix(string(u.State), adminEditStatePrefix) && cmd == "back" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "Р В Р ВµР Т‘Р В°Р С”РЎвЂљР С‘РЎР‚Р С•Р Р†Р В°Р Р…Р С‘Р Вµ Р С•РЎвЂљР СР ВµР Р…Р ВµР Р…Р С•.", keyboards.AdminSettingsEditorMenu())
		return
	}
	if cmd == "" && u.State == models.BotState(adminFAQAddState) && strings.TrimSpace(text) != "" {
		h.handleFAQAddInput(ctx, u, text)
		return
	}
	if cmd == "" && strings.HasPrefix(string(u.State), adminFAQEditStatePrefix) && strings.TrimSpace(text) != "" {
		h.handleFAQEditInput(ctx, u, text)
		return
	}
	if cmd == "" && u.State == models.BotState(adminQuestionAddState) && strings.TrimSpace(text) != "" {
		h.handleQuestionAddInput(ctx, u, text)
		return
	}
	if cmd == "" && strings.HasPrefix(string(u.State), adminQuestionEditStatePrefix) && strings.TrimSpace(text) != "" {
		h.handleQuestionEditInput(ctx, u, text)
		return
	}
	if cmd == "" && strings.HasPrefix(string(u.State), adminEditStatePrefix) && (strings.TrimSpace(text) != "" || len(msg.Attachments) > 0) {
		h.handleSettingInput(ctx, u, text, msg)
		return
	}

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
		h.base.send(ctx, u.VKID, fmt.Sprintf("СЂСџС™В« Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ [id%d] Р В·Р В°Р В±Р В»Р С•Р С”Р С‘РЎР‚Р С•Р Р†Р В°Р Р….", p.VKID), keyboards.AdminMenu())
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
			fmt.Sprintf("РІСњвЂћРїС‘РЏ [id%d] Р Р† Р С•РЎвЂ¦Р В»Р В°Р В¶Р Т‘Р ВµР Р…Р С‘Р С‘ Р Р…Р В° %d Р СР С‘Р Р…. (Р Т‘Р С• %s).", p.VKID, p.Mins, until.Format("15:04")),
			keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "cooldown", fmt.Sprintf("%dmin", p.Mins))
	case "admin_unban":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.Unban(ctx, p.VKID)
		h.base.send(ctx, u.VKID, fmt.Sprintf("РІСљвЂ¦ Р С›Р С–РЎР‚Р В°Р Р…Р С‘РЎвЂЎР ВµР Р…Р С‘Р Вµ РЎРѓР Р…РЎРЏРЎвЂљР С• РЎРѓ [id%d].", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "unban", "")
	case "admin_set_mod":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("СЂСџвЂВ® [id%d] Р Р…Р В°Р В·Р Р…Р В°РЎвЂЎР ВµР Р… Р СР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚Р С•Р С.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_mod", "")
	case "admin_set_user":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("СЂСџвЂВ¤ [id%d] РЎР‚Р В°Р В·Р В¶Р В°Р В»Р С•Р Р†Р В°Р Р… Р Т‘Р С• Р С—Р С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЏ.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_user", "")
	case "admin_set_limit":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.base.send(ctx, u.VKID,
			fmt.Sprintf("Р вЂ™Р Р†Р ВµР Т‘Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†РЎвЂ№Р в„– Р В»Р С‘Р СР С‘РЎвЂљ Р В·Р В°Р С—РЎР‚Р С•РЎРѓР С•Р Р† Р Т‘Р В»РЎРЏ [id%d]:\n/setlimit %d <РЎвЂЎР С‘РЎРѓР В»Р С•>", p.VKID, p.VKID),
			keyboards.AdminMenu())
	case "admin_metrics":
		h.handleMetrics(ctx, u)
	case "admin_settings":
		h.handleSettingsMenu(ctx, u)
	case "admin_edit_setting":
		h.handleEditSettingStart(ctx, u, msg.Payload)
	case "admin_manage_faq":
		h.handleFAQManage(ctx, u)
	case "admin_faq_pick":
		h.handleFAQPick(ctx, u, msg.Payload)
	case "admin_faq_add":
		h.userSvc.UpdateState(ctx, u.VKID, models.BotState(adminFAQAddState))
		h.base.send(ctx, u.VKID, "Р СџРЎР‚Р С‘РЎв‚¬Р В»Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†РЎвЂ№Р в„– Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ FAQ Р С•Р Т‘Р Р…Р С‘Р С РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р ВµР С.", keyboards.BackOnly())
	case "admin_faq_edit":
		h.handleFAQEditStart(ctx, u, msg.Payload)
	case "admin_faq_delete":
		h.handleFAQDelete(ctx, u, msg.Payload)
	case "admin_manage_questions":
		h.handleQuestionManage(ctx, u)
	case "admin_q_pick":
		h.handleQuestionPick(ctx, u, msg.Payload)
	case "admin_q_add":
		h.userSvc.UpdateState(ctx, u.VKID, models.BotState(adminQuestionAddState))
		h.base.send(ctx, u.VKID, "Р СџРЎР‚Р С‘РЎв‚¬Р В»Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†РЎвЂ№Р в„– Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ Р В°Р Р…Р С”Р ВµРЎвЂљРЎвЂ№ Р С•Р Т‘Р Р…Р С‘Р С РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р ВµР С.", keyboards.BackOnly())
	case "admin_q_edit":
		h.handleQuestionEditStart(ctx, u, msg.Payload)
	case "admin_q_delete":
		h.handleQuestionDelete(ctx, u, msg.Payload)
	case "admin_mods":
		h.handleModsMenu(ctx, u)
	case "admin_audit":
		h.handleAuditLogs(ctx, u, 20, 0)
	case "approve_request":
		h.handleAccessDecision(ctx, u, msg, true)
	case "reject_request":
		h.handleAccessDecision(ctx, u, msg, false)
	case "main_chat":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
		h.base.send(ctx, u.VKID, "Р вЂ”Р ВµРЎР‚Р С”Р В°Р В»Р С• Р Р†Р С”Р В»РЎР‹РЎвЂЎР ВµР Р…Р С•. Р СџР С‘РЎв‚¬Р С‘РЎвЂљР Вµ РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р Вµ.", keyboards.AdminChatMenu())
	case "map_chat":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMapChat)
		h.base.send(ctx, u.VKID, "Р С™Р В°РЎР‚РЎвЂљР В° Р Р†Р С”Р В»РЎР‹РЎвЂЎР ВµР Р…Р В°. Р СџР С‘РЎв‚¬Р С‘РЎвЂљР Вµ РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р Вµ.", keyboards.AdminChatMenu())
	case "admin_clear_mirror":
		h.clearAdminDialogHistory(ctx, u, models.DialogMain, "Р вЂ”Р ВµРЎР‚Р С”Р В°Р В»Р С•")
	case "admin_clear_map":
		h.clearAdminDialogHistory(ctx, u, models.DialogMap, "Р С™Р В°РЎР‚РЎвЂљР В°")
	case "support":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID, "СЂСџвЂєВ  Р В§Р В°РЎвЂљ Р С—Р С•Р Т‘Р Т‘Р ВµРЎР‚Р В¶Р С”Р С‘.", keyboards.BackOnly())
	case "admin_panel":
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "СЂСџвЂвЂ Р СџР В°Р Р…Р ВµР В»РЎРЉ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°", keyboards.AdminMenu())
	default:
		// Р вЂўРЎРѓР В»Р С‘ Р В°Р Т‘Р СР С‘Р Р… Р Р† РЎР‚Р ВµР В¶Р С‘Р СР Вµ Р Т‘Р С‘Р В°Р В»Р С•Р С–Р В° РІР‚вЂќ Р С•РЎвЂљР С—РЎР‚Р В°Р Р†Р В»РЎРЏР ВµР С Р Р† AI
		if u.State == models.StateMainChat || u.State == models.StateMapChat || u.State == models.StateSupport {
			h.handleAIChat(ctx, u, msg, cmd, text)
		} else {
			h.handleTextCommand(ctx, u, text)
		}
	}
}

func (h *AdminHandler) handleInviteMenu(ctx context.Context, u *models.User) {
	link, err := h.inviteSvc.Create(ctx, u.VKID, 1) // Р С•Р Т‘Р Р…Р С•РЎР‚Р В°Р В·Р С•Р Р†Р В°РЎРЏ
	if err != nil {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° РЎРѓР С•Р В·Р Т‘Р В°Р Р…Р С‘РЎРЏ РЎРѓРЎРѓРЎвЂ№Р В»Р С”Р С‘: "+err.Error(), keyboards.AdminMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("РІСљвЂ¦ Р СњР С•Р Р†Р В°РЎРЏ РЎРѓРЎРѓРЎвЂ№Р В»Р С”Р В°-Р С—РЎР‚Р С‘Р С–Р В»Р В°РЎв‚¬Р ВµР Р…Р С‘Р Вµ (Р С•Р Т‘Р Р…Р С•РЎР‚Р В°Р В·Р С•Р Р†Р В°РЎРЏ):\n%s\n\nР вЂќР ВµР в„–РЎРѓРЎвЂљР Р†РЎС“Р ВµРЎвЂљ 72 РЎвЂЎР В°РЎРѓР В°.", link.URL),
		keyboards.AdminMenu())
}

func (h *AdminHandler) handleUsersPage(ctx context.Context, u *models.User, offset int) {
	allUsers, err := h.userSvc.ListAll(ctx)
	if err != nil {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р С—Р С•Р В»РЎС“РЎвЂЎР ВµР Р…Р С‘РЎРЏ Р С—Р С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»Р ВµР в„–", keyboards.AdminMenu())
		return
	}
	if len(allUsers) == 0 {
		h.base.send(ctx, u.VKID, "Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»Р ВµР в„– Р Р…Р ВµРЎвЂљ.", keyboards.AdminMenu())
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
		roleIcon := "СЂСџвЂВ¤"
		if usr.Role == models.RoleModerator {
			roleIcon = "СЂСџвЂВ®"
		} else if usr.Role == models.RoleAdmin {
			roleIcon = "СЂСџвЂвЂ"
		}
		statusIcon := ""
		if usr.Status == models.StatusBanned || usr.Status == models.StatusRestricted {
			statusIcon = " СЂСџС™В«"
		}
		btns = append(btns, keyboards.UserButton{
			VKID: usr.VKID,
			Name: roleIcon + " " + name + statusIcon,
		})
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("СЂСџвЂТђ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»Р С‘ (%dРІР‚вЂњ%d Р С‘Р В· %d)\n\nР СњР В°Р В¶Р СР С‘РЎвЂљР Вµ Р Р…Р В° Р С—Р С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЏ:", offset+1, end, len(allUsers)),
		keyboards.UserListInline(btns, offset, len(allUsers)))
}

func (h *AdminHandler) handleUserDetail(ctx context.Context, admin *models.User, targetVKID int64) {
	target, err := h.userSvc.GetByVKID(ctx, targetVKID)
	if err != nil || target == nil {
		h.base.send(ctx, admin.VKID, "РІСњРЉ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", keyboards.AdminMenu())
		return
	}
	statusEmoji := "СЂСџСџСћ"
	switch target.Status {
	case models.StatusBanned:
		statusEmoji = "СЂСџвЂќТ‘"
	case models.StatusRestricted:
		statusEmoji = "СЂСџСџРЋ"
	case models.StatusPending:
		statusEmoji = "Р Р†РЎв„ўР вЂћ"
	}
	banInfo := "РІР‚вЂќ"
	if target.BannedUntil != nil {
		banInfo = target.BannedUntil.Format("02.01.2006 15:04")
	}
	limitStr := "Р Р†РІвЂљВ¬РЎвЂє"
	if target.RequestLimit > 0 {
		limitStr = fmt.Sprintf("%d", target.RequestLimit)
	}
	text := fmt.Sprintf(
		"СЂСџвЂВ¤ [id%d|%s %s]\n\n"+
			"Р В Р С•Р В»РЎРЉ: %s\n"+
			"%s Р РЋРЎвЂљР В°РЎвЂљРЎС“РЎРѓ: %s\n"+
			"СЂСџвЂ™В¬ Р вЂ”Р В°Р С—РЎР‚Р С•РЎРѓР С•Р Р†: %d / %s\n"+
			"СЂСџвЂ™В° Р вЂР В°Р В»Р В°Р Р…РЎРѓ: %.2f РІвЂљР…\n"+
			"СЂСџвЂўС’ Р С›Р С–РЎР‚Р В°Р Р…Р С‘РЎвЂЎР ВµР Р… Р Т‘Р С•: %s\n"+
			"СЂСџвЂњвЂ¦ Р В Р ВµР С–Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂ Р С‘РЎРЏ: %s",
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
		"СЂСџвЂњР‰ Р СљР С•Р Р…Р С‘РЎвЂљР С•РЎР‚Р С‘Р Р…Р С–\n\n"+
			"СЂСџвЂВ¤ Р С’Р С”РЎвЂљР С‘Р Р†Р Р…РЎвЂ№РЎвЂ¦: %d\n"+
			"СЂСџВ¤вЂ“ AI Р В·Р В°Р С—РЎР‚Р С•РЎРѓР С•Р Р† РЎРѓР ВµР С–Р С•Р Т‘Р Р…РЎРЏ: %d\n"+
			"РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С•Р С” РЎРѓР ВµР С–Р С•Р Т‘Р Р…РЎРЏ: %d\n"+
			"СЂСџвЂ™С• Р СџР В°Р СРЎРЏРЎвЂљРЎРЉ: %.1f Р СљР вЂ\n"+
			"РІРЏВ± Uptime: %s",
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
		"⚙️ Настройки бота\n\nВыберите параметр в меню ниже. После выбора отправьте новое значение одним сообщением.\n\nТекущее welcome:\n"+welcome,
		keyboards.AdminSettingsEditorMenu())
}

func (h *AdminHandler) handleModsMenu(ctx context.Context, u *models.User) {
	mods, _ := h.userSvc.ListAll(ctx)
	var sb strings.Builder
	sb.WriteString("СЂСџвЂВ® Р СљР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚РЎвЂ№:\n")
	count := 0
	for _, m := range mods {
		if m.Role == models.RoleModerator {
			sb.WriteString(fmt.Sprintf("РІР‚Сћ [id%d|%s %s]\n", m.VKID, m.FirstName, m.LastName))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("Р СњР ВµРЎвЂљ Р СР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚Р С•Р Р†\n")
	}
	sb.WriteString("\nР С™Р С•Р СР В°Р Р…Р Т‘РЎвЂ№:\n/addmod <vk_id> РІР‚вЂќ Р Т‘Р С•Р В±Р В°Р Р†Р С‘РЎвЂљРЎРЉ Р СР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°\n/delmod <vk_id> РІР‚вЂќ РЎС“Р Т‘Р В°Р В»Р С‘РЎвЂљРЎРЉ")
	h.base.send(ctx, u.VKID, sb.String(), keyboards.AdminMenu())
}

func (h *AdminHandler) handleAuditLogs(ctx context.Context, u *models.User, limit, offset int) {
	logs, err := h.settingsRepo.GetAuditLogs(ctx, limit, offset)
	if err != nil {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р В·Р В°Р С–РЎР‚РЎС“Р В·Р С”Р С‘ Р В°РЎС“Р Т‘Р С‘РЎвЂљР В°.", keyboards.AdminMenu())
		return
	}
	if len(logs) == 0 {
		h.base.send(ctx, u.VKID, "СЂСџвЂњСњ Р С’РЎС“Р Т‘Р С‘РЎвЂљ Р С—РЎС“РЎРѓРЎвЂљ.", keyboards.AdminMenu())
		return
	}

	var sb strings.Builder
	sb.WriteString("СЂСџвЂњСњ Р СџР С•РЎРѓР В»Р ВµР Т‘Р Р…Р С‘Р Вµ Р Т‘Р ВµР в„–РЎРѓРЎвЂљР Р†Р С‘РЎРЏ:\n\n")
	for i, l := range logs {
		target := "-"
		if l.TargetID != nil {
			target = fmt.Sprintf("%d", *l.TargetID)
		}
		sb.WriteString(fmt.Sprintf(
			"%d) %s | actor:%d | target:%s\n%s\n%s\n\n",
			i+1,
			l.CreatedAt.Format("02.01 15:04"),
			l.ActorID,
			target,
			l.Action,
			l.Details,
		))
	}
	h.base.send(ctx, u.VKID, sb.String(), keyboards.AdminMenu())
}

func (h *AdminHandler) handleFAQManage(ctx context.Context, u *models.User) {
	items, _ := h.getFAQItems(ctx)
	h.base.send(ctx, u.VKID, h.formatIndexedList("FAQ", items), h.buildFAQKeyboard(items))
}

func (h *AdminHandler) handleFAQPick(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", h.buildFAQKeyboard(nil))
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ РЎС“Р В¶Р Вµ Р С‘Р В·Р СР ВµР Р…Р ВµР Р… Р С‘Р В»Р С‘ РЎС“Р Т‘Р В°Р В»Р ВµР Р…. Р С›РЎвЂљР С”РЎР‚Р С•Р в„–РЎвЂљР Вµ РЎРѓР С—Р С‘РЎРѓР С•Р С” Р В·Р В°Р Р…Р С•Р Р†Р С•.", h.buildFAQKeyboard(items))
		return
	}
	kb := &keyboards.Keyboard{
		Inline: true,
		Buttons: [][]keyboards.Button{
			{
				keyboards.MakeBtn("РІСљРЏРїС‘РЏ Р В Р ВµР Т‘Р В°Р С”РЎвЂљР С‘РЎР‚Р С•Р Р†Р В°РЎвЂљРЎРЉ", "primary", fmt.Sprintf(`{"cmd":"admin_faq_edit","index":%d}`, idx)),
				keyboards.MakeBtn("СЂСџвЂ”вЂ Р Р€Р Т‘Р В°Р В»Р С‘РЎвЂљРЎРЉ", "negative", fmt.Sprintf(`{"cmd":"admin_faq_delete","index":%d}`, idx)),
			},
			{
				keyboards.MakeBtn("РІвЂ В©РїС‘РЏ Р С™ РЎРѓР С—Р С‘РЎРѓР С”РЎС“", "secondary", `{"cmd":"admin_manage_faq"}`),
			},
		},
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("РІСњвЂњ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ #%d:\n%s", idx+1, items[idx]), kb)
}

func (h *AdminHandler) handleFAQEditStart(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildFAQKeyboard(items))
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(fmt.Sprintf("%s%d", adminFAQEditStatePrefix, idx)))
	h.base.send(ctx, u.VKID, fmt.Sprintf("Р СћР ВµР С”РЎС“РЎвЂ°Р С‘Р в„– Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ:\n%s\n\nР СџРЎР‚Р С‘РЎв‚¬Р В»Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†РЎвЂ№Р в„– РЎвЂљР ВµР С”РЎРѓРЎвЂљ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓР В°.", items[idx]), keyboards.BackOnly())
}

func (h *AdminHandler) handleFAQDelete(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildFAQKeyboard(items))
		return
	}
	items = append(items[:idx], items[idx+1:]...)
	_ = h.saveFAQItems(ctx, items)
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "faq_delete", Details: fmt.Sprintf("index=%d", idx)})
	h.handleFAQManage(ctx, u)
}

func (h *AdminHandler) handleFAQAddInput(ctx context.Context, u *models.User, text string) {
	items, _ := h.getFAQItems(ctx)
	items = append(items, strings.TrimSpace(text))
	_ = h.saveFAQItems(ctx, items)
	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "faq_add"})
	h.handleFAQManage(ctx, u)
}

func (h *AdminHandler) handleFAQEditInput(ctx context.Context, u *models.User, text string) {
	idx, err := strconv.Atoi(strings.TrimPrefix(string(u.State), adminFAQEditStatePrefix))
	if err != nil {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р С‘Р Р…Р Т‘Р ВµР С”РЎРѓР В° Р Р†Р С•Р С—РЎР‚Р С•РЎРѓР В°.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildFAQKeyboard(items))
		return
	}
	items[idx] = strings.TrimSpace(text)
	_ = h.saveFAQItems(ctx, items)
	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "faq_edit", Details: fmt.Sprintf("index=%d", idx)})
	h.handleFAQManage(ctx, u)
}

func (h *AdminHandler) handleQuestionManage(ctx context.Context, u *models.User) {
	items, _ := h.getQuestionnaireItems(ctx)
	h.base.send(ctx, u.VKID, h.formatIndexedList("Р РЋРЎвЂљР В°РЎР‚РЎвЂљР С•Р Р†Р В°РЎРЏ Р В°Р Р…Р С”Р ВµРЎвЂљР В°", items), h.buildQuestionKeyboard(items))
}

func (h *AdminHandler) handleQuestionPick(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", h.buildQuestionKeyboard(nil))
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ РЎС“Р В¶Р Вµ Р С‘Р В·Р СР ВµР Р…Р ВµР Р… Р С‘Р В»Р С‘ РЎС“Р Т‘Р В°Р В»Р ВµР Р…. Р С›РЎвЂљР С”РЎР‚Р С•Р в„–РЎвЂљР Вµ РЎРѓР С—Р С‘РЎРѓР С•Р С” Р В·Р В°Р Р…Р С•Р Р†Р С•.", h.buildQuestionKeyboard(items))
		return
	}
	kb := &keyboards.Keyboard{
		Inline: true,
		Buttons: [][]keyboards.Button{
			{
				keyboards.MakeBtn("РІСљРЏРїС‘РЏ Р В Р ВµР Т‘Р В°Р С”РЎвЂљР С‘РЎР‚Р С•Р Р†Р В°РЎвЂљРЎРЉ", "primary", fmt.Sprintf(`{"cmd":"admin_q_edit","index":%d}`, idx)),
				keyboards.MakeBtn("СЂСџвЂ”вЂ Р Р€Р Т‘Р В°Р В»Р С‘РЎвЂљРЎРЉ", "negative", fmt.Sprintf(`{"cmd":"admin_q_delete","index":%d}`, idx)),
			},
			{
				keyboards.MakeBtn("РІвЂ В©РїС‘РЏ Р С™ РЎРѓР С—Р С‘РЎРѓР С”РЎС“", "secondary", `{"cmd":"admin_manage_questions"}`),
			},
		},
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("СЂСџвЂњСњ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ #%d:\n%s", idx+1, items[idx]), kb)
}

func (h *AdminHandler) handleQuestionEditStart(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildQuestionKeyboard(items))
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(fmt.Sprintf("%s%d", adminQuestionEditStatePrefix, idx)))
	h.base.send(ctx, u.VKID, fmt.Sprintf("Р СћР ВµР С”РЎС“РЎвЂ°Р С‘Р в„– Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ:\n%s\n\nР СџРЎР‚Р С‘РЎв‚¬Р В»Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†РЎвЂ№Р в„– РЎвЂљР ВµР С”РЎРѓРЎвЂљ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓР В°.", items[idx]), keyboards.BackOnly())
}

func (h *AdminHandler) handleQuestionDelete(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "РІСњРЉ Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р†Р С•Р С—РЎР‚Р С•РЎРѓ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildQuestionKeyboard(items))
		return
	}
	items = append(items[:idx], items[idx+1:]...)
	_ = h.saveQuestionnaireItems(ctx, items)
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "question_delete", Details: fmt.Sprintf("index=%d", idx)})
	h.handleQuestionManage(ctx, u)
}

func (h *AdminHandler) handleQuestionAddInput(ctx context.Context, u *models.User, text string) {
	items, _ := h.getQuestionnaireItems(ctx)
	items = append(items, strings.TrimSpace(text))
	_ = h.saveQuestionnaireItems(ctx, items)
	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "question_add"})
	h.handleQuestionManage(ctx, u)
}

func (h *AdminHandler) handleQuestionEditInput(ctx context.Context, u *models.User, text string) {
	idx, err := strconv.Atoi(strings.TrimPrefix(string(u.State), adminQuestionEditStatePrefix))
	if err != nil {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р С‘Р Р…Р Т‘Р ВµР С”РЎРѓР В° Р Р†Р С•Р С—РЎР‚Р С•РЎРѓР В°.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "РІС™В РїС‘РЏ Р вЂ™Р С•Р С—РЎР‚Р С•РЎРѓ Р Р…Р Вµ Р Р…Р В°Р в„–Р Т‘Р ВµР Р….", h.buildQuestionKeyboard(items))
		return
	}
	items[idx] = strings.TrimSpace(text)
	_ = h.saveQuestionnaireItems(ctx, items)
	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{ActorID: u.VKID, Action: "question_edit", Details: fmt.Sprintf("index=%d", idx)})
	h.handleQuestionManage(ctx, u)
}

func (h *AdminHandler) getFAQItems(ctx context.Context) ([]string, error) {
	return h.getStringListSetting(ctx, models.SettingFAQItems, defaultFAQItems())
}

func (h *AdminHandler) saveFAQItems(ctx context.Context, items []string) error {
	return h.saveStringListSetting(ctx, models.SettingFAQItems, items)
}

func (h *AdminHandler) getQuestionnaireItems(ctx context.Context) ([]string, error) {
	return h.getStringListSetting(ctx, models.SettingQuestionnaireItems, defaultQuestionnaireItems())
}

func (h *AdminHandler) saveQuestionnaireItems(ctx context.Context, items []string) error {
	return h.saveStringListSetting(ctx, models.SettingQuestionnaireItems, items)
}

func (h *AdminHandler) getStringListSetting(ctx context.Context, key string, fallback []string) ([]string, error) {
	raw, err := h.settingsRepo.Get(ctx, key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return append([]string{}, fallback...), nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return append([]string{}, fallback...), nil
	}
	filtered := make([]string, 0, len(items))
	for _, it := range items {
		t := strings.TrimSpace(it)
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return append([]string{}, fallback...), nil
	}
	return filtered, nil
}

func (h *AdminHandler) saveStringListSetting(ctx context.Context, key string, items []string) error {
	filtered := make([]string, 0, len(items))
	for _, it := range items {
		t := strings.TrimSpace(it)
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	return h.settingsRepo.Set(ctx, key, string(data))
}

func (h *AdminHandler) buildFAQKeyboard(items []string) *keyboards.Keyboard {
	kb := &keyboards.Keyboard{Inline: true}
	for i, q := range items {
		label := q
		if len(label) > 28 {
			label = label[:25] + "..."
		}
		kb.Buttons = append(kb.Buttons, []keyboards.Button{
			keyboards.MakeBtn(fmt.Sprintf("%d) %s", i+1, label), "secondary", fmt.Sprintf(`{"cmd":"admin_faq_pick","index":%d}`, i)),
		})
	}
	kb.Buttons = append(kb.Buttons, []keyboards.Button{
		keyboards.MakeBtn("вћ• Р”РѕР±Р°РІРёС‚СЊ РІРѕРїСЂРѕСЃ", "positive", `{"cmd":"admin_faq_add"}`),
		keyboards.MakeBtn("в†©пёЏ РќР°Р·Р°Рґ", "secondary", `{"cmd":"admin_settings"}`),
	})
	return kb
}

func (h *AdminHandler) buildQuestionKeyboard(items []string) *keyboards.Keyboard {
	kb := &keyboards.Keyboard{Inline: true}
	for i, q := range items {
		label := q
		if len(label) > 28 {
			label = label[:25] + "..."
		}
		kb.Buttons = append(kb.Buttons, []keyboards.Button{
			keyboards.MakeBtn(fmt.Sprintf("%d) %s", i+1, label), "secondary", fmt.Sprintf(`{"cmd":"admin_q_pick","index":%d}`, i)),
		})
	}
	kb.Buttons = append(kb.Buttons, []keyboards.Button{
		keyboards.MakeBtn("вћ• Р”РѕР±Р°РІРёС‚СЊ РІРѕРїСЂРѕСЃ", "positive", `{"cmd":"admin_q_add"}`),
		keyboards.MakeBtn("в†©пёЏ РќР°Р·Р°Рґ", "secondary", `{"cmd":"admin_settings"}`),
	})
	return kb
}

func (h *AdminHandler) formatIndexedList(title string, items []string) string {
	var sb strings.Builder
	sb.WriteString("РІС™в„ўРїС‘РЏ " + title + "\n\n")
	if len(items) == 0 {
		sb.WriteString("Р РЋР С—Р С‘РЎРѓР С•Р С” Р С—РЎС“РЎРѓРЎвЂљ.\n")
		return sb.String()
	}
	for i, q := range items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	sb.WriteString("\nР вЂ™РЎвЂ№Р В±Р ВµРЎР‚Р С‘РЎвЂљР Вµ Р С—РЎС“Р Р…Р С”РЎвЂљ Р С”Р Р…Р С•Р С—Р С”Р С•Р в„– Р Р…Р С‘Р В¶Р Вµ.")
	return sb.String()
}

func parseIndexPayload(payloadJSON string) (int, bool) {
	var p struct {
		Index int `json:"index"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return 0, false
	}
	return p.Index, true
}

func defaultFAQItems() []string {
	return []string{
		"Р С™Р В°Р С” Р Р…Р В°РЎвЂЎР В°РЎвЂљРЎРЉ Р С—Р С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљРЎРЉРЎРѓРЎРЏ Р В±Р С•РЎвЂљР С•Р С?",
		"Р С™Р В°Р С” Р С—Р С•Р С—Р С•Р В»Р Р…Р С‘РЎвЂљРЎРЉ Р В±Р В°Р В»Р В°Р Р…РЎРѓ?",
	}
}

func defaultQuestionnaireItems() []string {
	return []string{
		"Р С™Р В°Р С” Р Р†Р В°РЎРѓ Р В·Р С•Р Р†РЎС“РЎвЂљ?",
		"Р В§Р ВµР С Р Р†РЎвЂ№ Р В·Р В°Р Р…Р С‘Р СР В°Р ВµРЎвЂљР ВµРЎРѓРЎРЉ?",
		"Р С™Р В°Р С” Р Р†РЎвЂ№ РЎС“Р В·Р Р…Р В°Р В»Р С‘ Р С• Р Р…Р В°РЎРѓ?",
	}
}

func (h *AdminHandler) handleTextCommand(ctx context.Context, u *models.User, text string) {
	switch {
	case strings.HasPrefix(text, "/setwelcome "):
		msg := strings.TrimPrefix(text, "/setwelcome ")
		h.settingsRepo.Set(ctx, models.SettingWelcomeMessage, msg)
		h.base.send(ctx, u.VKID, "РІСљвЂ¦ Р СџРЎР‚Р С‘Р Р†Р ВµРЎвЂљРЎРѓРЎвЂљР Р†Р ВµР Р…Р Р…Р С•Р Вµ РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р Вµ Р С•Р В±Р Р…Р С•Р Р†Р В»Р ВµР Р…Р С•.", keyboards.AdminMenu())
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
		h.base.send(ctx, u.VKID, fmt.Sprintf("РІСљвЂ¦ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ %d РЎР‚Р В°Р В·Р В±Р В»Р С•Р С”Р С‘РЎР‚Р С•Р Р†Р В°Р Р….", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/addmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("РІСљвЂ¦ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ %d Р Р…Р В°Р В·Р Р…Р В°РЎвЂЎР ВµР Р… Р СР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚Р С•Р С.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/delmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("РІСљвЂ¦ Р СљР С•Р Т‘Р ВµРЎР‚Р В°РЎвЂљР С•РЎР‚ %d Р С—Р С•Р Р…Р С‘Р В¶Р ВµР Р… Р Т‘Р С• Р С—Р С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЏ.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/setlimit "):
		// /setlimit <vk_id> <count>
		parts := strings.Fields(text)
		if len(parts) < 3 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		limit, _ := strconv.Atoi(parts[2])
		h.userSvc.SetRequestLimit(ctx, targetID, limit)
		h.base.send(ctx, u.VKID, fmt.Sprintf("РІСљвЂ¦ Р вЂєР С‘Р СР С‘РЎвЂљ %d Р В·Р В°Р С—РЎР‚Р С•РЎРѓР С•Р Р† РЎС“РЎРѓРЎвЂљР В°Р Р…Р С•Р Р†Р В»Р ВµР Р… Р Т‘Р В»РЎРЏ %d.", limit, targetID), keyboards.AdminMenu())

	case strings.EqualFold(strings.TrimSpace(text), "/clear_mirror"):
		h.clearAdminDialogHistory(ctx, u, models.DialogMain, "Р вЂ”Р ВµРЎР‚Р С”Р В°Р В»Р С•")

	case strings.EqualFold(strings.TrimSpace(text), "/clear_map"):
		h.clearAdminDialogHistory(ctx, u, models.DialogMap, "Р С™Р В°РЎР‚РЎвЂљР В°")

	default:
		// Р СџР С•Р С”Р В°Р В·РЎвЂ№Р Р†Р В°Р ВµР С Р СР ВµР Р…РЎР‹ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°
		h.base.send(ctx, u.VKID, "СЂСџвЂвЂ Р СџР В°Р Р…Р ВµР В»РЎРЉ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°", keyboards.AdminMenu())
	}
}

func (h *AdminHandler) handleEditSettingStart(ctx context.Context, u *models.User, payloadJSON string) {
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil || strings.TrimSpace(p.Key) == "" {
		h.base.send(ctx, u.VKID, "Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р Р…Р В°РЎРѓРЎвЂљРЎР‚Р С•Р в„–Р С”РЎС“.", keyboards.AdminSettingsEditorMenu())
		return
	}
	if !isEditableSettingKey(p.Key) {
		h.base.send(ctx, u.VKID, "Р В­РЎвЂљРЎС“ Р Р…Р В°РЎРѓРЎвЂљРЎР‚Р С•Р в„–Р С”РЎС“ Р Р…Р ВµР В»РЎРЉР В·РЎРЏ Р СР ВµР Р…РЎРЏРЎвЂљРЎРЉ Р С‘Р В· Р С—Р В°Р Р…Р ВµР В»Р С‘.", keyboards.AdminSettingsEditorMenu())
		return
	}

	current, _ := h.settingsRepo.Get(ctx, p.Key)
	if fileName := promptFileNameBySetting(p.Key); fileName != "" && strings.TrimSpace(current) == "" {
		if filePrompt, err := os.ReadFile(fileName); err == nil && strings.TrimSpace(string(filePrompt)) != "" {
			current = strings.TrimSpace(string(filePrompt))
		} else {
			current = fmt.Sprintf("(Р С—РЎС“РЎРѓРЎвЂљР С•: РЎРѓР ВµР в„–РЎвЂЎР В°РЎРѓ Р С‘РЎРѓР С—Р С•Р В»РЎРЉР В·РЎС“Р ВµРЎвЂљРЎРѓРЎРЏ Р С—РЎР‚Р С•Р СР С—РЎвЂљ Р С‘Р В· РЎвЂћР В°Р в„–Р В»Р В° %s)", fileName)
		}
	}

	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(adminEditStatePrefix+p.Key))
	h.base.send(
		ctx,
		u.VKID,
		fmt.Sprintf("Р В Р ВµР Т‘Р В°Р С”РЎвЂљР С‘РЎР‚Р С•Р Р†Р В°Р Р…Р С‘Р Вµ `%s`\nР СћР ВµР С”РЎС“РЎвЂ°Р ВµР Вµ Р В·Р Р…Р В°РЎвЂЎР ВµР Р…Р С‘Р Вµ:\n%s\n\nР СџРЎР‚Р С‘РЎв‚¬Р В»Р С‘РЎвЂљР Вµ Р Р…Р С•Р Р†Р С•Р Вµ Р В·Р Р…Р В°РЎвЂЎР ВµР Р…Р С‘Р Вµ Р С•Р Т‘Р Р…Р С‘Р С РЎРѓР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р ВµР С Р С‘Р В»Р С‘ TXT-РЎвЂћР В°Р в„–Р В»Р С•Р С.", p.Key, current),
		keyboards.BackOnly(),
	)
}

func (h *AdminHandler) handleSettingInput(ctx context.Context, u *models.User, text string, msg object.MessagesMessage) {
	key := strings.TrimSpace(strings.TrimPrefix(string(u.State), adminEditStatePrefix))
	value := strings.TrimSpace(text)
	if key == "" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•Р С—РЎР‚Р ВµР Т‘Р ВµР В»Р С‘РЎвЂљРЎРЉ Р С”Р В»РЎР‹РЎвЂЎ Р Р…Р В°РЎРѓРЎвЂљРЎР‚Р С•Р в„–Р С”Р С‘.", keyboards.AdminMenu())
		return
	}

	if isPromptSettingKey(key) {
		if fileValue, ok := extractTxtAttachmentContent(msg); ok {
			value = strings.TrimSpace(fileValue)
		}
	}

	if value == "" {
		if isPromptSettingKey(key) {
			h.base.send(ctx, u.VKID, "Р С›РЎвЂљР С—РЎР‚Р В°Р Р†РЎРЉРЎвЂљР Вµ РЎвЂљР ВµР С”РЎРѓРЎвЂљ Р С‘Р В»Р С‘ TXT-РЎвЂћР В°Р в„–Р В» РЎРѓ Р Р…Р С•Р Р†РЎвЂ№Р С Р С—РЎР‚Р С•Р СР С—РЎвЂљР С•Р С.", keyboards.BackOnly())
			return
		}
		h.base.send(ctx, u.VKID, "Р СџРЎС“РЎРѓРЎвЂљР С•Р Вµ Р В·Р Р…Р В°РЎвЂЎР ВµР Р…Р С‘Р Вµ Р Р…Р Вµ РЎРѓР С•РЎвЂ¦РЎР‚Р В°Р Р…РЎРЏР ВµРЎвЂљРЎРѓРЎРЏ.", keyboards.BackOnly())
		return
	}

	if err := h.settingsRepo.Set(ctx, key, value); err != nil {
		h.base.send(ctx, u.VKID, "Р С›РЎв‚¬Р С‘Р В±Р С”Р В° РЎРѓР С•РЎвЂ¦РЎР‚Р В°Р Р…Р ВµР Р…Р С‘РЎРЏ.", keyboards.AdminMenu())
		return
	}

	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID: u.VKID,
		Action:  "set_setting",
		Details: fmt.Sprintf("key=%s", key),
	})
	h.base.send(ctx, u.VKID, fmt.Sprintf("Р СњР В°РЎРѓРЎвЂљРЎР‚Р С•Р в„–Р С”Р В° `%s` Р С•Р В±Р Р…Р С•Р Р†Р В»Р ВµР Р…Р В°.", key), keyboards.AdminSettingsEditorMenu())
}

func extractTxtAttachmentContent(msg object.MessagesMessage) (string, bool) {
	for _, a := range msg.Attachments {
		if a.Type != "doc" {
			continue
		}
		if !strings.EqualFold(a.Doc.Ext, "txt") || strings.TrimSpace(a.Doc.URL) == "" {
			continue
		}
		resp, err := http.Get(a.Doc.URL)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		return string(body), true
	}
	return "", false
}

func isEditableSettingKey(key string) bool {
	switch key {
	case models.SettingWelcomeMessage,
		models.SettingConsentText,
		models.SettingFAQText,
		models.SettingAboutText,
		models.SettingSystemPrompt,
		models.SettingGamePrompt,
		models.SettingMapPrompt,
		models.SettingDefaultRequestLimit,
		models.SettingDefaultCooldownSecs:
		return true
	default:
		return false
	}
}

func isPromptSettingKey(key string) bool {
	switch key {
	case models.SettingSystemPrompt,
		models.SettingGamePrompt,
		models.SettingMapPrompt:
		return true
	default:
		return false
	}
}

func promptFileNameBySetting(key string) string {
	switch key {
	case models.SettingSystemPrompt:
		return "system_prompt.txt"
	case models.SettingGamePrompt:
		return "system_prompt_game.txt"
	case models.SettingMapPrompt:
		return "system_prompt_map.txt"
	default:
		return ""
	}
}

func (h *AdminHandler) handleAccessDecision(ctx context.Context, actor *models.User, msg object.MessagesMessage, approve bool) {
	// Р В§Р С‘РЎвЂљР В°Р ВµР С req_id Р С‘ vk_id Р С‘Р В· payload Р С”Р Р…Р С•Р С—Р С”Р С‘
	var payload struct {
		ReqID int64 `json:"req_id"`
		VKID  int64 `json:"vk_id"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ReqID == 0 {
		h.base.send(ctx, actor.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В°: Р Р…Р Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С—РЎР‚Р С•РЎвЂЎР С‘РЎвЂљР В°РЎвЂљРЎРЉ Р Т‘Р В°Р Р…Р Р…РЎвЂ№Р Вµ Р В·Р В°РЎРЏР Р†Р С”Р С‘.", keyboards.AdminMenu())
		return
	}

	if approve {
		applicantVKID, err := h.userSvc.ApproveAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р С•Р Т‘Р С•Р В±РЎР‚Р ВµР Р…Р С‘РЎРЏ: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("РІСљвЂ¦ Р вЂ”Р В°РЎРЏР Р†Р С”Р В° #%d Р С•Р Т‘Р С•Р В±РЎР‚Р ВµР Р…Р В°. Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ [id%d|Р В°Р С”РЎвЂљР С‘Р Р†Р С‘РЎР‚Р С•Р Р†Р В°Р Р…].", payload.ReqID, applicantVKID),
			keyboards.AdminMenu())
		// Р Р€Р Р†Р ВµР Т‘Р С•Р СР В»РЎРЏР ВµР С Р В·Р В°РЎРЏР Р†Р С‘РЎвЂљР ВµР В»РЎРЏ
		h.base.send(ctx, applicantVKID,
			"СЂСџР‹вЂ° Р вЂ™Р В°РЎв‚¬Р В° Р В·Р В°РЎРЏР Р†Р С”Р В° Р С•Р Т‘Р С•Р В±РЎР‚Р ВµР Р…Р В°! Р СњР В°Р С—Р С‘РЎв‚¬Р С‘РЎвЂљР Вµ РЎвЂЎРЎвЂљР С•-Р Р…Р С‘Р В±РЎС“Р Т‘РЎРЉ РЎвЂЎРЎвЂљР С•Р В±РЎвЂ№ Р Р…Р В°РЎвЂЎР В°РЎвЂљРЎРЉ.",
			keyboards.Empty())
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "approve_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	} else {
		applicantVKID, err := h.userSvc.RejectAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "РІСњРЉ Р С›РЎв‚¬Р С‘Р В±Р С”Р В° Р С•РЎвЂљР С”Р В»Р С•Р Р…Р ВµР Р…Р С‘РЎРЏ: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("РІСњРЉ Р вЂ”Р В°РЎРЏР Р†Р С”Р В° #%d Р С•РЎвЂљР С”Р В»Р С•Р Р…Р ВµР Р…Р В°.", payload.ReqID),
			keyboards.AdminMenu())
		// Р Р€Р Р†Р ВµР Т‘Р С•Р СР В»РЎРЏР ВµР С Р В·Р В°РЎРЏР Р†Р С‘РЎвЂљР ВµР В»РЎРЏ
		if applicantVKID > 0 {
			h.base.send(ctx, applicantVKID,
				"СЂСџВвЂќ Р вЂ™Р В°РЎв‚¬Р В° Р В·Р В°РЎРЏР Р†Р С”Р В° Р Р…Р В° Р Р†РЎРѓРЎвЂљРЎС“Р С—Р В»Р ВµР Р…Р С‘Р Вµ Р С•РЎвЂљР С”Р В»Р С•Р Р…Р ВµР Р…Р В°.",
				keyboards.Empty())
		}
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "reject_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	}
}

func (h *AdminHandler) handleBanCommand(ctx context.Context, u *models.User, text string, isCooldown bool) {
	// /ban <vk_id> Р С‘Р В»Р С‘ /cool <vk_id> <minutes>
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
	msg := fmt.Sprintf("РІСљвЂ¦ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ %d Р В·Р В°Р В±Р В»Р С•Р С”Р С‘РЎР‚Р С•Р Р†Р В°Р Р….", targetID)
	if until != nil {
		msg = fmt.Sprintf("РІСњвЂћРїС‘РЏ Р СџР С•Р В»РЎРЉР В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉ %d Р Р† Р С•РЎвЂ¦Р В»Р В°Р В¶Р Т‘Р ВµР Р…Р С‘Р С‘ Р Т‘Р С• %s.", targetID, until.Format("02.01 15:04"))
	}
	h.base.send(ctx, u.VKID, msg, keyboards.AdminMenu())
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID: u.VKID, TargetID: &targetID, Action: "ban", Details: text,
	})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dРЎвЂЎ %dР С", h, m)
}

func (h *AdminHandler) logAudit(ctx context.Context, actorID, targetID int64, action, details string) {
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID:  actorID,
		TargetID: &targetID,
		Action:   action,
		Details:  details,
	})
}

// handleAIChat РІР‚вЂќ AI-Р Т‘Р С‘Р В°Р В»Р С•Р С– Р Т‘Р В»РЎРЏ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В° (Р В°Р Р…Р В°Р В»Р С•Р С– user.go, Р Р…Р С• Р В±Р ВµР В· Р В»Р С‘Р СР С‘РЎвЂљР С•Р Р†)
func (h *AdminHandler) handleAIChat(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if cmd == "back" || cmd == "admin_panel" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "СЂСџвЂвЂ Р СџР В°Р Р…Р ВµР В»РЎРЉ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°", keyboards.AdminMenu())
		return
	}
	switch strings.TrimSpace(strings.ToLower(text)) {
	case strings.ToLower("Р С›РЎвЂЎР С‘РЎРѓРЎвЂљР С‘РЎвЂљРЎРЉ Р вЂ”Р ВµРЎР‚Р С”Р В°Р В»Р С•"):
		h.clearAdminDialogHistory(ctx, u, models.DialogMain, "Р вЂ”Р ВµРЎР‚Р С”Р В°Р В»Р С•")
		return
	case strings.ToLower("Р С›РЎвЂЎР С‘РЎРѓРЎвЂљР С‘РЎвЂљРЎРЉ Р С™Р В°РЎР‚РЎвЂљРЎС“"):
		h.clearAdminDialogHistory(ctx, u, models.DialogMap, "Р С™Р В°РЎР‚РЎвЂљР В°")
		return
	}
	if u.State == models.StateSupport {
		if cmd == "back" {
			h.userSvc.UpdateState(ctx, u.VKID, "")
			h.base.send(ctx, u.VKID, "СЂСџвЂвЂ Р СџР В°Р Р…Р ВµР В»РЎРЉ Р В°Р Т‘Р СР С‘Р Р…Р С‘РЎРѓРЎвЂљРЎР‚Р В°РЎвЂљР С•РЎР‚Р В°", keyboards.AdminMenu())
			return
		}
		dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
		h.dialogRepo.SaveMessage(ctx, &models.Message{
			DialogID: dialog.ID, UserID: u.ID,
			Role: models.MessageRoleUser, Type: models.MessageTypeText, Content: text,
		})
		h.base.send(ctx, u.VKID, "РІСљвЂ¦ Р РЋР С•Р С•Р В±РЎвЂ°Р ВµР Р…Р С‘Р Вµ РЎРѓР С•РЎвЂ¦РЎР‚Р В°Р Р…Р ВµР Р…Р С• Р Р† Р С—Р С•Р Т‘Р Т‘Р ВµРЎР‚Р В¶Р С”Р Вµ.", keyboards.BackOnly())
		return
	}

	if strings.TrimSpace(text) == "" {
		return
	}

	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, h.adminDialogTypeForState(u.State))
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
	if sp := h.loadAdminPromptForState(ctx, u.State); strings.TrimSpace(sp) != "" {
		aiMessages = append([]models.AIMessage{{Role: "system", Content: sp}}, aiMessages...)
	}
	aiMessages = appendScenarioGuard(aiMessages)

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("admin ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "вљ пёЏ РћС€РёР±РєР° AI: "+err.Error(), keyboards.AdminChatMenu())
		return
	}

	reply, err = repairScenarioReply(ctx, h.aiSvc, aiMessages, text, reply)
	if err != nil {
		slog.Error("admin repair ai reply", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "вљ пёЏ РћС€РёР±РєР° AI: "+err.Error(), keyboards.AdminChatMenu())
		return
	}

	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID, UserID: u.ID,
		Role: models.MessageRoleAssistant, Type: models.MessageTypeText, Content: reply,
	})

	h.base.send(ctx, u.VKID, reply, keyboards.AdminChatMenu())
}

func (h *AdminHandler) loadAdminPromptForState(ctx context.Context, state models.BotState) string {
	switch state {
	case models.StateMapChat:
		if prompt := h.loadAdminPromptValue(ctx, models.SettingMapPrompt, "system_prompt_map.txt"); prompt != "" {
			return prompt
		}
	case models.StateMainChat:
		if prompt := h.loadAdminPromptValue(ctx, models.SettingGamePrompt, "system_prompt_game.txt"); prompt != "" {
			return prompt
		}
	}

	if prompt := h.loadAdminPromptValue(ctx, models.SettingSystemPrompt, "system_prompt.txt"); prompt != "" {
		return prompt
	}
	return ""
}

func (h *AdminHandler) loadAdminPromptValue(ctx context.Context, settingKey, fileName string) string {
	if raw, _ := h.settingsRepo.Get(ctx, settingKey); strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if data, err := os.ReadFile(fileName); err == nil && strings.TrimSpace(string(data)) != "" {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (h *AdminHandler) adminDialogTypeForState(state models.BotState) models.DialogType {
	switch state {
	case models.StateMapChat:
		return models.DialogMap
	default:
		return models.DialogMain
	}
}

func (h *AdminHandler) clearAdminDialogHistory(ctx context.Context, u *models.User, dtype models.DialogType, title string) {
	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, dtype)
	if err != nil {
		h.base.send(ctx, u.VKID, "Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С—Р С•Р В»РЎС“РЎвЂЎР С‘РЎвЂљРЎРЉ Р Т‘Р С‘Р В°Р В»Р С•Р С– Р Т‘Р В»РЎРЏ Р С•РЎвЂЎР С‘РЎРѓРЎвЂљР С”Р С‘.", keyboards.AdminChatMenu())
		return
	}
	if err := h.dialogRepo.ClearHistory(ctx, dialog.ID); err != nil {
		h.base.send(ctx, u.VKID, "Р СњР Вµ РЎС“Р Т‘Р В°Р В»Р С•РЎРѓРЎРЉ Р С•РЎвЂЎР С‘РЎРѓРЎвЂљР С‘РЎвЂљРЎРЉ Р С‘РЎРѓРЎвЂљР С•РЎР‚Р С‘РЎР‹.", keyboards.AdminChatMenu())
		return
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("Р ВРЎРѓРЎвЂљР С•РЎР‚Р С‘РЎРЏ РЎвЂЎР В°РЎвЂљР В° \"%s\" Р С•РЎвЂЎР С‘РЎвЂ°Р ВµР Р…Р В°.", title), keyboards.AdminChatMenu())
}
