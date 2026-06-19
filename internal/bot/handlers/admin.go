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

// AdminHandler вЂ” РѕР±СЂР°Р±РѕС‚С‡РёРє РєРѕРјР°РЅРґ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°
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
		h.base.send(ctx, u.VKID, "Р РµРґР°РєС‚РёСЂРѕРІР°РЅРёРµ РѕС‚РјРµРЅРµРЅРѕ.", keyboards.AdminSettingsEditorMenu())
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
		h.base.send(ctx, u.VKID, fmt.Sprintf("рџљ« РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ [id%d] Р·Р°Р±Р»РѕРєРёСЂРѕРІР°РЅ.", p.VKID), keyboards.AdminMenu())
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
			fmt.Sprintf("вќ„пёЏ [id%d] РІ РѕС…Р»Р°Р¶РґРµРЅРёРё РЅР° %d РјРёРЅ. (РґРѕ %s).", p.VKID, p.Mins, until.Format("15:04")),
			keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "cooldown", fmt.Sprintf("%dmin", p.Mins))
	case "admin_unban":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.Unban(ctx, p.VKID)
		h.base.send(ctx, u.VKID, fmt.Sprintf("вњ… РћРіСЂР°РЅРёС‡РµРЅРёРµ СЃРЅСЏС‚Рѕ СЃ [id%d].", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "unban", "")
	case "admin_set_mod":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("рџ‘® [id%d] РЅР°Р·РЅР°С‡РµРЅ РјРѕРґРµСЂР°С‚РѕСЂРѕРј.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_mod", "")
	case "admin_set_user":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.userSvc.SetRole(ctx, p.VKID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("рџ‘¤ [id%d] СЂР°Р·Р¶Р°Р»РѕРІР°РЅ РґРѕ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ.", p.VKID), keyboards.AdminMenu())
		h.logAudit(ctx, u.VKID, p.VKID, "set_user", "")
	case "admin_set_limit":
		var p struct {
			VKID int64 `json:"vk_id"`
		}
		json.Unmarshal([]byte(msg.Payload), &p)
		h.base.send(ctx, u.VKID,
			fmt.Sprintf("Р’РІРµРґРёС‚Рµ РЅРѕРІС‹Р№ Р»РёРјРёС‚ Р·Р°РїСЂРѕСЃРѕРІ РґР»СЏ [id%d]:\n/setlimit %d <С‡РёСЃР»Рѕ>", p.VKID, p.VKID),
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
		h.base.send(ctx, u.VKID, "РџСЂРёС€Р»РёС‚Рµ РЅРѕРІС‹Р№ РІРѕРїСЂРѕСЃ FAQ РѕРґРЅРёРј СЃРѕРѕР±С‰РµРЅРёРµРј.", keyboards.BackOnly())
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
		h.base.send(ctx, u.VKID, "РџСЂРёС€Р»РёС‚Рµ РЅРѕРІС‹Р№ РІРѕРїСЂРѕСЃ Р°РЅРєРµС‚С‹ РѕРґРЅРёРј СЃРѕРѕР±С‰РµРЅРёРµРј.", keyboards.BackOnly())
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
		h.base.send(ctx, u.VKID, "рџ’¬ Р РµР¶РёРј РґРёР°Р»РѕРіР° СЃ AI. РџРёС€РёС‚Рµ РІРѕРїСЂРѕСЃ.", keyboards.AdminChatMenu())
	case "support":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID, "рџ›  Р§Р°С‚ РїРѕРґРґРµСЂР¶РєРё.", keyboards.BackOnly())
	case "admin_panel":
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "рџ‘‘ РџР°РЅРµР»СЊ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°", keyboards.AdminMenu())
	default:
		// Р•СЃР»Рё Р°РґРјРёРЅ РІ СЂРµР¶РёРјРµ РґРёР°Р»РѕРіР° вЂ” РѕС‚РїСЂР°РІР»СЏРµРј РІ AI
		if u.State == models.StateMainChat || u.State == models.StateSupport {
			h.handleAIChat(ctx, u, msg, cmd, text)
		} else {
			h.handleTextCommand(ctx, u, text)
		}
	}
}

func (h *AdminHandler) handleInviteMenu(ctx context.Context, u *models.User) {
	link, err := h.inviteSvc.Create(ctx, u.VKID, 1) // РѕРґРЅРѕСЂР°Р·РѕРІР°СЏ
	if err != nil {
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° СЃРѕР·РґР°РЅРёСЏ СЃСЃС‹Р»РєРё: "+err.Error(), keyboards.AdminMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("вњ… РќРѕРІР°СЏ СЃСЃС‹Р»РєР°-РїСЂРёРіР»Р°С€РµРЅРёРµ (РѕРґРЅРѕСЂР°Р·РѕРІР°СЏ):\n%s\n\nР”РµР№СЃС‚РІСѓРµС‚ 72 С‡Р°СЃР°.", link.URL),
		keyboards.AdminMenu())
}

func (h *AdminHandler) handleUsersPage(ctx context.Context, u *models.User, offset int) {
	allUsers, err := h.userSvc.ListAll(ctx)
	if err != nil {
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° РїРѕР»СѓС‡РµРЅРёСЏ РїРѕР»СЊР·РѕРІР°С‚РµР»РµР№", keyboards.AdminMenu())
		return
	}
	if len(allUsers) == 0 {
		h.base.send(ctx, u.VKID, "РџРѕР»СЊР·РѕРІР°С‚РµР»РµР№ РЅРµС‚.", keyboards.AdminMenu())
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
		roleIcon := "рџ‘¤"
		if usr.Role == models.RoleModerator {
			roleIcon = "рџ‘®"
		} else if usr.Role == models.RoleAdmin {
			roleIcon = "рџ‘‘"
		}
		statusIcon := ""
		if usr.Status == models.StatusBanned || usr.Status == models.StatusRestricted {
			statusIcon = " рџљ«"
		}
		btns = append(btns, keyboards.UserButton{
			VKID: usr.VKID,
			Name: roleIcon + " " + name + statusIcon,
		})
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("рџ‘Ґ РџРѕР»СЊР·РѕРІР°С‚РµР»Рё (%dвЂ“%d РёР· %d)\n\nРќР°Р¶РјРёС‚Рµ РЅР° РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ:", offset+1, end, len(allUsers)),
		keyboards.UserListInline(btns, offset, len(allUsers)))
}

func (h *AdminHandler) handleUserDetail(ctx context.Context, admin *models.User, targetVKID int64) {
	target, err := h.userSvc.GetByVKID(ctx, targetVKID)
	if err != nil || target == nil {
		h.base.send(ctx, admin.VKID, "вќЊ РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ РЅРµ РЅР°Р№РґРµРЅ.", keyboards.AdminMenu())
		return
	}
	statusEmoji := "рџџў"
	switch target.Status {
	case models.StatusBanned:
		statusEmoji = "рџ”ґ"
	case models.StatusRestricted:
		statusEmoji = "рџџЎ"
	case models.StatusPending:
		statusEmoji = "вљЄ"
	}
	banInfo := "вЂ”"
	if target.BannedUntil != nil {
		banInfo = target.BannedUntil.Format("02.01.2006 15:04")
	}
	limitStr := "в€ћ"
	if target.RequestLimit > 0 {
		limitStr = fmt.Sprintf("%d", target.RequestLimit)
	}
	text := fmt.Sprintf(
		"рџ‘¤ [id%d|%s %s]\n\n"+
			"Р РѕР»СЊ: %s\n"+
			"%s РЎС‚Р°С‚СѓСЃ: %s\n"+
			"рџ’¬ Р—Р°РїСЂРѕСЃРѕРІ: %d / %s\n"+
			"рџ’° Р‘Р°Р»Р°РЅСЃ: %.2f в‚Ѕ\n"+
			"рџ•ђ РћРіСЂР°РЅРёС‡РµРЅ РґРѕ: %s\n"+
			"рџ“… Р РµРіРёСЃС‚СЂР°С†РёСЏ: %s",
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
		"рџ“Љ РњРѕРЅРёС‚РѕСЂРёРЅРі\n\n"+
			"рџ‘¤ РђРєС‚РёРІРЅС‹С…: %d\n"+
			"рџ¤– AI Р·Р°РїСЂРѕСЃРѕРІ СЃРµРіРѕРґРЅСЏ: %d\n"+
			"вќЊ РћС€РёР±РѕРє СЃРµРіРѕРґРЅСЏ: %d\n"+
			"рџ’ѕ РџР°РјСЏС‚СЊ: %.1f РњР‘\n"+
			"вЏ± Uptime: %s",
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
		welcome = "(РЅРµ Р·Р°РґР°РЅРѕ)"
	}
	h.base.send(ctx, u.VKID,
		"вљ™пёЏ РќР°СЃС‚СЂРѕР№РєРё Р±РѕС‚Р°\n\nР’С‹Р±РµСЂРёС‚Рµ РїР°СЂР°РјРµС‚СЂ РІ РјРµРЅСЋ РЅРёР¶Рµ. РџРѕСЃР»Рµ РІС‹Р±РѕСЂР° РѕС‚РїСЂР°РІСЊС‚Рµ РЅРѕРІРѕРµ Р·РЅР°С‡РµРЅРёРµ РѕРґРЅРёРј СЃРѕРѕР±С‰РµРЅРёРµРј.\n\nРўРµРєСѓС‰РµРµ welcome:\n"+welcome,
		keyboards.AdminSettingsEditorMenu())
}

func (h *AdminHandler) handleModsMenu(ctx context.Context, u *models.User) {
	mods, _ := h.userSvc.ListAll(ctx)
	var sb strings.Builder
	sb.WriteString("рџ‘® РњРѕРґРµСЂР°С‚РѕСЂС‹:\n")
	count := 0
	for _, m := range mods {
		if m.Role == models.RoleModerator {
			sb.WriteString(fmt.Sprintf("вЂў [id%d|%s %s]\n", m.VKID, m.FirstName, m.LastName))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("РќРµС‚ РјРѕРґРµСЂР°С‚РѕСЂРѕРІ\n")
	}
	sb.WriteString("\nРљРѕРјР°РЅРґС‹:\n/addmod <vk_id> вЂ” РґРѕР±Р°РІРёС‚СЊ РјРѕРґРµСЂР°С‚РѕСЂР°\n/delmod <vk_id> вЂ” СѓРґР°Р»РёС‚СЊ")
	h.base.send(ctx, u.VKID, sb.String(), keyboards.AdminMenu())
}

func (h *AdminHandler) handleAuditLogs(ctx context.Context, u *models.User, limit, offset int) {
	logs, err := h.settingsRepo.GetAuditLogs(ctx, limit, offset)
	if err != nil {
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° Р·Р°РіСЂСѓР·РєРё Р°СѓРґРёС‚Р°.", keyboards.AdminMenu())
		return
	}
	if len(logs) == 0 {
		h.base.send(ctx, u.VKID, "рџ“ќ РђСѓРґРёС‚ РїСѓСЃС‚.", keyboards.AdminMenu())
		return
	}

	var sb strings.Builder
	sb.WriteString("рџ“ќ РџРѕСЃР»РµРґРЅРёРµ РґРµР№СЃС‚РІРёСЏ:\n\n")
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
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", h.buildFAQKeyboard(nil))
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ СѓР¶Рµ РёР·РјРµРЅРµРЅ РёР»Рё СѓРґР°Р»РµРЅ. РћС‚РєСЂРѕР№С‚Рµ СЃРїРёСЃРѕРє Р·Р°РЅРѕРІРѕ.", h.buildFAQKeyboard(items))
		return
	}
	kb := &keyboards.Keyboard{
		Inline: true,
		Buttons: [][]keyboards.Button{
			{
				keyboards.MakeBtn("вњЏпёЏ Р РµРґР°РєС‚РёСЂРѕРІР°С‚СЊ", "primary", fmt.Sprintf(`{"cmd":"admin_faq_edit","index":%d}`, idx)),
				keyboards.MakeBtn("рџ—‘ РЈРґР°Р»РёС‚СЊ", "negative", fmt.Sprintf(`{"cmd":"admin_faq_delete","index":%d}`, idx)),
			},
			{
				keyboards.MakeBtn("в†©пёЏ Рљ СЃРїРёСЃРєСѓ", "secondary", `{"cmd":"admin_manage_faq"}`),
			},
		},
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("вќ“ Р’РѕРїСЂРѕСЃ #%d:\n%s", idx+1, items[idx]), kb)
}

func (h *AdminHandler) handleFAQEditStart(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildFAQKeyboard(items))
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(fmt.Sprintf("%s%d", adminFAQEditStatePrefix, idx)))
	h.base.send(ctx, u.VKID, fmt.Sprintf("РўРµРєСѓС‰РёР№ РІРѕРїСЂРѕСЃ:\n%s\n\nРџСЂРёС€Р»РёС‚Рµ РЅРѕРІС‹Р№ С‚РµРєСЃС‚ РІРѕРїСЂРѕСЃР°.", items[idx]), keyboards.BackOnly())
}

func (h *AdminHandler) handleFAQDelete(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildFAQKeyboard(items))
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
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° РёРЅРґРµРєСЃР° РІРѕРїСЂРѕСЃР°.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getFAQItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildFAQKeyboard(items))
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
	h.base.send(ctx, u.VKID, h.formatIndexedList("РЎС‚Р°СЂС‚РѕРІР°СЏ Р°РЅРєРµС‚Р°", items), h.buildQuestionKeyboard(items))
}

func (h *AdminHandler) handleQuestionPick(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", h.buildQuestionKeyboard(nil))
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ СѓР¶Рµ РёР·РјРµРЅРµРЅ РёР»Рё СѓРґР°Р»РµРЅ. РћС‚РєСЂРѕР№С‚Рµ СЃРїРёСЃРѕРє Р·Р°РЅРѕРІРѕ.", h.buildQuestionKeyboard(items))
		return
	}
	kb := &keyboards.Keyboard{
		Inline: true,
		Buttons: [][]keyboards.Button{
			{
				keyboards.MakeBtn("вњЏпёЏ Р РµРґР°РєС‚РёСЂРѕРІР°С‚СЊ", "primary", fmt.Sprintf(`{"cmd":"admin_q_edit","index":%d}`, idx)),
				keyboards.MakeBtn("рџ—‘ РЈРґР°Р»РёС‚СЊ", "negative", fmt.Sprintf(`{"cmd":"admin_q_delete","index":%d}`, idx)),
			},
			{
				keyboards.MakeBtn("в†©пёЏ Рљ СЃРїРёСЃРєСѓ", "secondary", `{"cmd":"admin_manage_questions"}`),
			},
		},
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("рџ“ќ Р’РѕРїСЂРѕСЃ #%d:\n%s", idx+1, items[idx]), kb)
}

func (h *AdminHandler) handleQuestionEditStart(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildQuestionKeyboard(items))
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(fmt.Sprintf("%s%d", adminQuestionEditStatePrefix, idx)))
	h.base.send(ctx, u.VKID, fmt.Sprintf("РўРµРєСѓС‰РёР№ РІРѕРїСЂРѕСЃ:\n%s\n\nРџСЂРёС€Р»РёС‚Рµ РЅРѕРІС‹Р№ С‚РµРєСЃС‚ РІРѕРїСЂРѕСЃР°.", items[idx]), keyboards.BackOnly())
}

func (h *AdminHandler) handleQuestionDelete(ctx context.Context, u *models.User, payloadJSON string) {
	idx, ok := parseIndexPayload(payloadJSON)
	if !ok {
		h.base.send(ctx, u.VKID, "вќЊ РќРµ СѓРґР°Р»РѕСЃСЊ РѕРїСЂРµРґРµР»РёС‚СЊ РІРѕРїСЂРѕСЃ.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildQuestionKeyboard(items))
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
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° РёРЅРґРµРєСЃР° РІРѕРїСЂРѕСЃР°.", keyboards.AdminSettingsEditorMenu())
		return
	}
	items, _ := h.getQuestionnaireItems(ctx)
	if idx < 0 || idx >= len(items) {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "вљ пёЏ Р’РѕРїСЂРѕСЃ РЅРµ РЅР°Р№РґРµРЅ.", h.buildQuestionKeyboard(items))
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
	sb.WriteString("вљ™пёЏ " + title + "\n\n")
	if len(items) == 0 {
		sb.WriteString("РЎРїРёСЃРѕРє РїСѓСЃС‚.\n")
		return sb.String()
	}
	for i, q := range items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	sb.WriteString("\nР’С‹Р±РµСЂРёС‚Рµ РїСѓРЅРєС‚ РєРЅРѕРїРєРѕР№ РЅРёР¶Рµ.")
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
		"РљР°Рє РЅР°С‡Р°С‚СЊ РїРѕР»СЊР·РѕРІР°С‚СЊСЃСЏ Р±РѕС‚РѕРј?",
		"РљР°Рє РїРѕРїРѕР»РЅРёС‚СЊ Р±Р°Р»Р°РЅСЃ?",
	}
}

func defaultQuestionnaireItems() []string {
	return []string{
		"РљР°Рє РІР°СЃ Р·РѕРІСѓС‚?",
		"Р§РµРј РІС‹ Р·Р°РЅРёРјР°РµС‚РµСЃСЊ?",
		"РљР°Рє РІС‹ СѓР·РЅР°Р»Рё Рѕ РЅР°СЃ?",
	}
}

func (h *AdminHandler) handleTextCommand(ctx context.Context, u *models.User, text string) {
	switch {
	case strings.HasPrefix(text, "/setwelcome "):
		msg := strings.TrimPrefix(text, "/setwelcome ")
		h.settingsRepo.Set(ctx, models.SettingWelcomeMessage, msg)
		h.base.send(ctx, u.VKID, "вњ… РџСЂРёРІРµС‚СЃС‚РІРµРЅРЅРѕРµ СЃРѕРѕР±С‰РµРЅРёРµ РѕР±РЅРѕРІР»РµРЅРѕ.", keyboards.AdminMenu())
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
		h.base.send(ctx, u.VKID, fmt.Sprintf("вњ… РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ %d СЂР°Р·Р±Р»РѕРєРёСЂРѕРІР°РЅ.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/addmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleModerator)
		h.base.send(ctx, u.VKID, fmt.Sprintf("вњ… РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ %d РЅР°Р·РЅР°С‡РµРЅ РјРѕРґРµСЂР°С‚РѕСЂРѕРј.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/delmod "):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.userSvc.SetRole(ctx, targetID, models.RoleUser)
		h.base.send(ctx, u.VKID, fmt.Sprintf("вњ… РњРѕРґРµСЂР°С‚РѕСЂ %d РїРѕРЅРёР¶РµРЅ РґРѕ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ.", targetID), keyboards.AdminMenu())

	case strings.HasPrefix(text, "/setlimit "):
		// /setlimit <vk_id> <count>
		parts := strings.Fields(text)
		if len(parts) < 3 {
			break
		}
		targetID, _ := strconv.ParseInt(parts[1], 10, 64)
		limit, _ := strconv.Atoi(parts[2])
		h.userSvc.SetRequestLimit(ctx, targetID, limit)
		h.base.send(ctx, u.VKID, fmt.Sprintf("вњ… Р›РёРјРёС‚ %d Р·Р°РїСЂРѕСЃРѕРІ СѓСЃС‚Р°РЅРѕРІР»РµРЅ РґР»СЏ %d.", limit, targetID), keyboards.AdminMenu())

	default:
		// РџРѕРєР°Р·С‹РІР°РµРј РјРµРЅСЋ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°
		h.base.send(ctx, u.VKID, "рџ‘‘ РџР°РЅРµР»СЊ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°", keyboards.AdminMenu())
	}
}

func (h *AdminHandler) handleEditSettingStart(ctx context.Context, u *models.User, payloadJSON string) {
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil || strings.TrimSpace(p.Key) == "" {
		h.base.send(ctx, u.VKID, "Не удалось определить настройку.", keyboards.AdminSettingsEditorMenu())
		return
	}
	if !isEditableSettingKey(p.Key) {
		h.base.send(ctx, u.VKID, "Эту настройку нельзя менять из панели.", keyboards.AdminSettingsEditorMenu())
		return
	}

	current, _ := h.settingsRepo.Get(ctx, p.Key)
	if fileName := promptFileNameBySetting(p.Key); fileName != "" && strings.TrimSpace(current) == "" {
		if filePrompt, err := os.ReadFile(fileName); err == nil && strings.TrimSpace(string(filePrompt)) != "" {
			current = strings.TrimSpace(string(filePrompt))
		} else {
			current = fmt.Sprintf("(пусто: сейчас используется промпт из файла %s)", fileName)
		}
	}

	h.userSvc.UpdateState(ctx, u.VKID, models.BotState(adminEditStatePrefix+p.Key))
	h.base.send(
		ctx,
		u.VKID,
		fmt.Sprintf("Редактирование `%s`\nТекущее значение:\n%s\n\nПришлите новое значение одним сообщением или TXT-файлом.", p.Key, current),
		keyboards.BackOnly(),
	)
}

func (h *AdminHandler) handleSettingInput(ctx context.Context, u *models.User, text string, msg object.MessagesMessage) {
	key := strings.TrimSpace(strings.TrimPrefix(string(u.State), adminEditStatePrefix))
	value := strings.TrimSpace(text)
	if key == "" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "Не удалось определить ключ настройки.", keyboards.AdminMenu())
		return
	}

	if isPromptSettingKey(key) {
		if fileValue, ok := extractTxtAttachmentContent(msg); ok {
			value = strings.TrimSpace(fileValue)
		}
	}

	if value == "" {
		if isPromptSettingKey(key) {
			h.base.send(ctx, u.VKID, "Отправьте текст или TXT-файл с новым промптом.", keyboards.BackOnly())
			return
		}
		h.base.send(ctx, u.VKID, "Пустое значение не сохраняется.", keyboards.BackOnly())
		return
	}

	if err := h.settingsRepo.Set(ctx, key, value); err != nil {
		h.base.send(ctx, u.VKID, "Ошибка сохранения.", keyboards.AdminMenu())
		return
	}

	h.userSvc.UpdateState(ctx, u.VKID, "")
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID: u.VKID,
		Action:  "set_setting",
		Details: fmt.Sprintf("key=%s", key),
	})
	h.base.send(ctx, u.VKID, fmt.Sprintf("Настройка `%s` обновлена.", key), keyboards.AdminSettingsEditorMenu())
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
	// Р§РёС‚Р°РµРј req_id Рё vk_id РёР· payload РєРЅРѕРїРєРё
	var payload struct {
		ReqID int64 `json:"req_id"`
		VKID  int64 `json:"vk_id"`
	}
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ReqID == 0 {
		h.base.send(ctx, actor.VKID, "вќЊ РћС€РёР±РєР°: РЅРµ СѓРґР°Р»РѕСЃСЊ РїСЂРѕС‡РёС‚Р°С‚СЊ РґР°РЅРЅС‹Рµ Р·Р°СЏРІРєРё.", keyboards.AdminMenu())
		return
	}

	if approve {
		applicantVKID, err := h.userSvc.ApproveAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "вќЊ РћС€РёР±РєР° РѕРґРѕР±СЂРµРЅРёСЏ: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("вњ… Р—Р°СЏРІРєР° #%d РѕРґРѕР±СЂРµРЅР°. РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ [id%d|Р°РєС‚РёРІРёСЂРѕРІР°РЅ].", payload.ReqID, applicantVKID),
			keyboards.AdminMenu())
		// РЈРІРµРґРѕРјР»СЏРµРј Р·Р°СЏРІРёС‚РµР»СЏ
		h.base.send(ctx, applicantVKID,
			"рџЋ‰ Р’Р°С€Р° Р·Р°СЏРІРєР° РѕРґРѕР±СЂРµРЅР°! РќР°РїРёС€РёС‚Рµ С‡С‚Рѕ-РЅРёР±СѓРґСЊ С‡С‚РѕР±С‹ РЅР°С‡Р°С‚СЊ.",
			keyboards.Empty())
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "approve_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	} else {
		applicantVKID, err := h.userSvc.RejectAccessRequest(ctx, payload.ReqID)
		if err != nil {
			h.base.send(ctx, actor.VKID, "вќЊ РћС€РёР±РєР° РѕС‚РєР»РѕРЅРµРЅРёСЏ: "+err.Error(), keyboards.AdminMenu())
			return
		}
		h.base.send(ctx, actor.VKID,
			fmt.Sprintf("вќЊ Р—Р°СЏРІРєР° #%d РѕС‚РєР»РѕРЅРµРЅР°.", payload.ReqID),
			keyboards.AdminMenu())
		// РЈРІРµРґРѕРјР»СЏРµРј Р·Р°СЏРІРёС‚РµР»СЏ
		if applicantVKID > 0 {
			h.base.send(ctx, applicantVKID,
				"рџ” Р’Р°С€Р° Р·Р°СЏРІРєР° РЅР° РІСЃС‚СѓРїР»РµРЅРёРµ РѕС‚РєР»РѕРЅРµРЅР°.",
				keyboards.Empty())
		}
		h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
			ActorID: actor.VKID, TargetID: &applicantVKID, Action: "reject_request",
			Details: fmt.Sprintf("req_id=%d", payload.ReqID),
		})
	}
}

func (h *AdminHandler) handleBanCommand(ctx context.Context, u *models.User, text string, isCooldown bool) {
	// /ban <vk_id> РёР»Рё /cool <vk_id> <minutes>
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
	msg := fmt.Sprintf("вњ… РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ %d Р·Р°Р±Р»РѕРєРёСЂРѕРІР°РЅ.", targetID)
	if until != nil {
		msg = fmt.Sprintf("вќ„пёЏ РџРѕР»СЊР·РѕРІР°С‚РµР»СЊ %d РІ РѕС…Р»Р°Р¶РґРµРЅРёРё РґРѕ %s.", targetID, until.Format("02.01 15:04"))
	}
	h.base.send(ctx, u.VKID, msg, keyboards.AdminMenu())
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID: u.VKID, TargetID: &targetID, Action: "ban", Details: text,
	})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dС‡ %dРј", h, m)
}

func (h *AdminHandler) logAudit(ctx context.Context, actorID, targetID int64, action, details string) {
	h.settingsRepo.WriteAuditLog(ctx, &models.AuditLog{
		ActorID:  actorID,
		TargetID: &targetID,
		Action:   action,
		Details:  details,
	})
}

// handleAIChat вЂ” AI-РґРёР°Р»РѕРі РґР»СЏ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР° (Р°РЅР°Р»РѕРі user.go, РЅРѕ Р±РµР· Р»РёРјРёС‚РѕРІ)
func (h *AdminHandler) handleAIChat(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if cmd == "back" || cmd == "admin_panel" {
		h.userSvc.UpdateState(ctx, u.VKID, "")
		h.base.send(ctx, u.VKID, "рџ‘‘ РџР°РЅРµР»СЊ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°", keyboards.AdminMenu())
		return
	}
	if u.State == models.StateSupport {
		if cmd == "back" {
			h.userSvc.UpdateState(ctx, u.VKID, "")
			h.base.send(ctx, u.VKID, "рџ‘‘ РџР°РЅРµР»СЊ Р°РґРјРёРЅРёСЃС‚СЂР°С‚РѕСЂР°", keyboards.AdminMenu())
			return
		}
		dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
		h.dialogRepo.SaveMessage(ctx, &models.Message{
			DialogID: dialog.ID, UserID: u.ID,
			Role: models.MessageRoleUser, Type: models.MessageTypeText, Content: text,
		})
		h.base.send(ctx, u.VKID, "вњ… РЎРѕРѕР±С‰РµРЅРёРµ СЃРѕС…СЂР°РЅРµРЅРѕ РІ РїРѕРґРґРµСЂР¶РєРµ.", keyboards.BackOnly())
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
	if sp, _ := h.settingsRepo.Get(ctx, models.SettingSystemPrompt); strings.TrimSpace(sp) != "" {
		aiMessages = append([]models.AIMessage{{Role: "system", Content: sp}}, aiMessages...)
	}

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("admin ai complete", "err", err)
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
