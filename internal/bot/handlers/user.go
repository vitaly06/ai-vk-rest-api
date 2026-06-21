package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/SevereCloud/vksdk/v3/object"
	"github.com/vitaly06/ai-vk-bot/internal/bot/keyboards"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
	aiSvc "github.com/vitaly06/ai-vk-bot/internal/services/ai"
	monSvc "github.com/vitaly06/ai-vk-bot/internal/services/monitoring"
	paymentSvc "github.com/vitaly06/ai-vk-bot/internal/services/payment"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"
)

var questionnaire = []string{
	"РљР°Рє РІР°СЃ Р·РѕРІСѓС‚?",
	"Р§РµРј РІС‹ Р·Р°РЅРёРјР°РµС‚РµСЃСЊ?",
	"РљР°Рє РІС‹ СѓР·РЅР°Р»Рё Рѕ РЅР°СЃ?",
}

type UserHandler struct {
	base         *baseHandler
	userSvc      *userSvc.Service
	aiSvc        *aiSvc.Service
	paymentSvc   *paymentSvc.Service
	dialogRepo   repository.DialogRepository
	settingsRepo repository.SettingsRepository
	mon          *monSvc.Service
}

func (h *UserHandler) Handle(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch u.State {
	case models.StateWelcome:
		h.handleWelcome(ctx, u)
	case models.StateConsent:
		h.handleConsent(ctx, u, cmd)
	case models.StateQuestionnaire:
		h.handleQuestionnaire(ctx, u, text)
	case models.StateSupport:
		h.handleSupportDialog(ctx, u, msg, cmd, text)
	default:
		h.handleMainState(ctx, u, msg, cmd, text)
	}
}

func (h *UserHandler) handleWelcome(ctx context.Context, u *models.User) {
	welcome, _ := h.settingsRepo.Get(ctx, models.SettingWelcomeMessage)
	if welcome == "" {
		welcome = "Р”РѕР±СЂРѕ РїРѕР¶Р°Р»РѕРІР°С‚СЊ! РЇ РІР°С€ AI-РїРѕРјРѕС‰РЅРёРє."
	}

	consentText, _ := h.settingsRepo.Get(ctx, models.SettingConsentText)
	if consentText == "" {
		consentText = "Р”Р»СЏ РїСЂРѕРґРѕР»Р¶РµРЅРёСЏ РЅРµРѕР±С…РѕРґРёРјРѕ СЃРѕРіР»Р°СЃРёРµ РЅР° РѕР±СЂР°Р±РѕС‚РєСѓ РїРµСЂСЃРѕРЅР°Р»СЊРЅС‹С… РґР°РЅРЅС‹С… Рё РїРѕР»СѓС‡РµРЅРёРµ СЂР°СЃСЃС‹Р»РѕРє."
	}

	h.base.send(ctx, u.VKID, welcome+"\n\n"+consentText, keyboards.ConsentKeyboard())
	_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateConsent)
}

func (h *UserHandler) handleConsent(ctx context.Context, u *models.User, cmd string) {
	switch cmd {
	case "consent_accept":
		_ = h.userSvc.SaveConsent(ctx, u.VKID, true, false)
		h.base.send(ctx, u.VKID, "РҐРѕС‚РёС‚Рµ РїРѕР»СѓС‡Р°С‚СЊ РїРѕР»РµР·РЅС‹Рµ СЂР°СЃСЃС‹Р»РєРё?", keyboards.MailingConsentKeyboard())
	case "mailing_yes":
		_ = h.userSvc.SaveConsent(ctx, u.VKID, true, true)
		h.startQuestionnaire(ctx, u)
	case "mailing_no":
		h.startQuestionnaire(ctx, u)
	case "consent_decline":
		h.base.send(ctx, u.VKID, "Р‘РµР· СЃРѕРіР»Р°СЃРёСЏ РїРѕР»СЊР·РѕРІР°С‚СЊСЃСЏ Р±РѕС‚РѕРј РЅРµР»СЊР·СЏ.", keyboards.Empty())
	}
}

func (h *UserHandler) startQuestionnaire(ctx context.Context, u *models.User) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
		h.base.send(ctx, u.VKID, "Р’С‹Р±РµСЂРёС‚Рµ СЂРµР¶РёРј: Р—РµСЂРєР°Р»Рѕ РёР»Рё РљР°СЂС‚Р°.", keyboards.MainMenu())
		return
	}
	_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateQuestionnaire)
	h.base.send(ctx, u.VKID, "РќРµР±РѕР»СЊС€Р°СЏ Р°РЅРєРµС‚Р°. "+questions[0], keyboards.Empty())
}

func (h *UserHandler) handleQuestionnaire(ctx context.Context, u *models.User, text string) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
		h.base.send(ctx, u.VKID, "Р’С‹Р±РµСЂРёС‚Рµ СЂРµР¶РёРј: Р—РµСЂРєР°Р»Рѕ РёР»Рё РљР°СЂС‚Р°.", keyboards.MainMenu())
		return
	}

	answers, _ := h.userSvc.GetQAnswers(ctx, u.ID)
	currentQ := len(answers)

	if currentQ < len(questions) {
		_ = h.userSvc.SaveQAnswer(ctx, u.ID, questions[currentQ], text)
		currentQ++
	}

	if currentQ < len(questions) {
		h.base.send(ctx, u.VKID, questions[currentQ], keyboards.Empty())
		return
	}

	_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
	h.base.send(ctx, u.VKID, "РђРЅРєРµС‚Р° Р·Р°РїРѕР»РЅРµРЅР°. РўРµРїРµСЂСЊ РјРѕР¶РЅРѕ РІС‹Р±СЂР°С‚СЊ СЂРµР¶РёРј.", keyboards.MainMenu())
}

func (h *UserHandler) handleMainState(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "game_chat":
		h.enterScenarioMode(ctx, u, models.StateMainChat)
	case "map_chat":
		h.enterScenarioMode(ctx, u, models.StateMapChat)
	case "support":
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID, "Р§Р°С‚ С‚РµС…РЅРёС‡РµСЃРєРѕР№ РїРѕРґРґРµСЂР¶РєРё. РћРїРёС€РёС‚Рµ РїСЂРѕР±Р»РµРјСѓ.", keyboards.BackOnly())
	case "payment":
		h.handlePaymentMenu(ctx, u)
	case "services":
		h.handleServices(ctx, u)
	case "profile":
		h.handleProfile(ctx, u)
	case "faq":
		h.base.send(ctx, u.VKID, h.renderFAQ(ctx), keyboards.MainMenu())
	case "pay_card":
		h.initiatePayment(ctx, u)
	case "pay_wallet":
		h.handleWalletPayment(ctx, u)
	default:
		if u.State == models.StateMainChat || u.State == models.StateMapChat {
			h.handleAIChat(ctx, u, text)
			return
		}
		h.base.send(ctx, u.VKID, "Р’С‹Р±РµСЂРёС‚Рµ СЂРµР¶РёРј: Р—РµСЂРєР°Р»Рѕ РёР»Рё РљР°СЂС‚Р°.", keyboards.MainMenu())
	}
}

func (h *UserHandler) enterScenarioMode(ctx context.Context, u *models.User, state models.BotState) {
	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, h.dialogTypeForState(state))
	if err != nil {
		slog.Error("get dialog for mode", "err", err)
		h.base.send(ctx, u.VKID, "РќРµ СѓРґР°Р»РѕСЃСЊ РѕС‚РєСЂС‹С‚СЊ СЂРµР¶РёРј. РџРѕРїСЂРѕР±СѓР№С‚Рµ РїРѕР·Р¶Рµ.", keyboards.MainMenu())
		return
	}
	_ = h.userSvc.UpdateState(ctx, u.VKID, state)

	history, err := h.dialogRepo.GetHistory(ctx, dialog.ID, 1)
	if err == nil && len(history) > 0 {
		if state == models.StateMapChat {
			h.base.send(ctx, u.VKID, "РљР°СЂС‚Р° РѕС‚РєСЂС‹С‚Р°. РџСЂРѕРґРѕР»Р¶Р°РµРј СЌС‚Сѓ РІРµС‚РєСѓ.", keyboards.MainMenu())
			return
		}
		h.base.send(ctx, u.VKID, "Р—РµСЂРєР°Р»Рѕ РѕС‚РєСЂС‹С‚Рѕ. РџСЂРѕРґРѕР»Р¶Р°РµРј СЌС‚Сѓ РІРµС‚РєСѓ.", keyboards.MainMenu())
		return
	}

	reply, err := h.generateModeIntro(ctx, state)
	if err != nil {
		slog.Error("generate mode intro", "state", state, "err", err)
		h.base.send(ctx, u.VKID, "РќРµ СѓРґР°Р»РѕСЃСЊ Р·Р°РїСѓСЃС‚РёС‚СЊ СЂРµР¶РёРј. РџСЂРѕРІРµСЂСЊС‚Рµ РїСЂРѕРјРїС‚.", keyboards.MainMenu())
		return
	}

	_, _ = h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleAssistant,
		Type:     models.MessageTypeText,
		Content:  reply,
	})
	h.base.send(ctx, u.VKID, reply, keyboards.MainMenu())
}

func (h *UserHandler) generateModeIntro(ctx context.Context, state models.BotState) (string, error) {
	prompt := h.loadPromptForState(ctx, state)
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("empty prompt for state %s", state)
	}

	startMessage := "РќР°С‡РЅРё РґРёР°Р»РѕРі РїРѕ РїСЂР°РІРёР»Р°Рј СЌС‚РѕРіРѕ СЂРµР¶РёРјР°."
	if state == models.StateMainChat {
		startMessage = "РќР°С‡РЅРё СЃРёРјСѓР»СЏС†РёСЋ РїРѕ РїСЂР°РІРёР»Р°Рј СЂРµР¶РёРјР°. РЎРЅР°С‡Р°Р»Р° Р·Р°РїСЂРѕСЃРё РЅРµРѕР±С…РѕРґРёРјС‹Рµ РґР°РЅРЅС‹Рµ Рё Р·Р°С‚РµРј РґР°Р№ РїРµСЂРІС‹Р№ С…РѕРґ СЃСЂРµРґС‹."
	}
	if state == models.StateMapChat {
		startMessage = "РќР°С‡РЅРё СЂРµР¶РёРј РєР°СЂС‚С‹. РЎРЅР°С‡Р°Р»Р° Р·Р°РїСЂРѕСЃРё РёСЃС…РѕРґРЅС‹Рµ РґР°РЅРЅС‹Рµ, РЅСѓР¶РЅС‹Рµ РґР»СЏ С‡С‚РµРЅРёСЏ РєР°СЂС‚С‹, Рё РІРµРґРё РґРёР°Р»РѕРі РІ СЂР°РјРєР°С… РёРЅСЃС‚СЂСѓРєС†РёРё."
	}

	messages := []models.AIMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: startMessage},
	}
	messages = appendScenarioGuard(messages)

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, messages)
	if err != nil {
		return "", err
	}
	return repairScenarioReply(ctx, h.aiSvc, messages, startMessage, reply)
}

func (h *UserHandler) handleAIChat(ctx context.Context, u *models.User, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	ok, err := h.userSvc.CheckLimit(ctx, u)
	if err != nil {
		slog.Error("check limit", "err", err)
	}
	if !ok {
		h.base.send(ctx, u.VKID, "Р’С‹ РґРѕСЃС‚РёРіР»Рё Р»РёРјРёС‚Р° Р·Р°РїСЂРѕСЃРѕРІ. РџРѕРїРѕР»РЅРёС‚Рµ Р±Р°Р»Р°РЅСЃ РёР»Рё РґРѕР¶РґРёС‚РµСЃСЊ СЃР±СЂРѕСЃР° Р»РёРјРёС‚Р°.", keyboards.MainMenu())
		return
	}

	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, h.dialogTypeForState(u.State))
	if err != nil {
		slog.Error("get dialog", "err", err)
		h.mon.RecordError()
		return
	}

	_, _ = h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})

	history, err := h.dialogRepo.GetHistory(ctx, dialog.ID, 20)
	if err != nil {
		slog.Error("get history", "err", err)
	}

	aiMessages := make([]models.AIMessage, 0, len(history)+1)
	prompt := h.loadPromptForState(ctx, u.State)
	if strings.TrimSpace(prompt) != "" {
		aiMessages = append(aiMessages, models.AIMessage{Role: "system", Content: prompt})
	}
	for _, m := range history {
		role := "user"
		if m.Role == models.MessageRoleAssistant {
			role = "assistant"
		}
		aiMessages = append(aiMessages, models.AIMessage{Role: role, Content: m.Content})
	}
	aiMessages = appendScenarioGuard(aiMessages)

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "РћС€РёР±РєР° AI. РџРѕРїСЂРѕР±СѓР№С‚Рµ РїРѕР·Р¶Рµ.", keyboards.MainMenu())
		return
	}
	reply, err = repairScenarioReply(ctx, h.aiSvc, aiMessages, text, reply)
	if err != nil {
		slog.Error("repair ai reply", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "РћС€РёР±РєР° AI. РџРѕРїСЂРѕР±СѓР№С‚Рµ РїРѕР·Р¶Рµ.", keyboards.MainMenu())
		return
	}

	_, _ = h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleAssistant,
		Type:     models.MessageTypeText,
		Content:  reply,
	})

	_ = h.userSvc.IncrRequestCount(ctx, u.VKID)
	h.base.send(ctx, u.VKID, reply, keyboards.MainMenu())
}

func (h *UserHandler) handleSupportDialog(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if cmd == "back" {
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
		h.base.send(ctx, u.VKID, "Р’С‹ РІРµСЂРЅСѓР»РёСЃСЊ РІ РјРµРЅСЋ.", keyboards.MainMenu())
		return
	}

	dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
	_, _ = h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})
	h.base.send(ctx, u.VKID, "РЎРѕРѕР±С‰РµРЅРёРµ РѕС‚РїСЂР°РІР»РµРЅРѕ РІ РїРѕРґРґРµСЂР¶РєСѓ. РњС‹ РѕС‚РІРµС‚РёРј РїРѕР·Р¶Рµ.", keyboards.BackOnly())
}

func (h *UserHandler) handlePaymentMenu(ctx context.Context, u *models.User) {
	info := fmt.Sprintf("Р’Р°С€ Р±Р°Р»Р°РЅСЃ: %.2f RUB\n\nР’С‹Р±РµСЂРёС‚Рµ СЃРїРѕСЃРѕР± РїРѕРїРѕР»РЅРµРЅРёСЏ:", u.Balance)
	h.base.send(ctx, u.VKID, info, keyboards.PaymentMethods())
}

func (h *UserHandler) initiatePayment(ctx context.Context, u *models.User) {
	p, err := h.paymentSvc.CreatePayment(ctx, u.ID, 100, "РџРѕРїРѕР»РЅРµРЅРёРµ Р±Р°Р»Р°РЅСЃР°")
	if err != nil {
		h.base.send(ctx, u.VKID, "РћС€РёР±РєР° РїСЂРё СЃРѕР·РґР°РЅРёРё РїР»Р°С‚РµР¶Р°.", keyboards.MainMenu())
		return
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("РЎСЃС‹Р»РєР° РґР»СЏ РѕРїР»Р°С‚С‹:\n%s", p.ConfirmationURL), keyboards.MainMenu())
}

func (h *UserHandler) handleWalletPayment(ctx context.Context, u *models.User) {
	h.base.send(ctx, u.VKID, fmt.Sprintf("Р‘Р°Р»Р°РЅСЃ РєРѕС€РµР»СЊРєР°: %.2f RUB\n\nР¤СѓРЅРєС†РёСЏ РІ СЂР°Р·СЂР°Р±РѕС‚РєРµ.", u.Balance), keyboards.MainMenu())
}

func (h *UserHandler) handleServices(ctx context.Context, u *models.User) {
	products, err := h.paymentSvc.ListProducts(ctx)
	if err != nil || len(products) == 0 {
		h.base.send(ctx, u.VKID, "РљР°С‚Р°Р»РѕРі СѓСЃР»СѓРі РїРѕРєР° РїСѓСЃС‚.", keyboards.MainMenu())
		return
	}
	var sb strings.Builder
	sb.WriteString("РЈСЃР»СѓРіРё Рё С†РµРЅС‹:\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("- %s вЂ” %.0f RUB\n  %s\n\n", p.Name, p.Price, p.Description))
	}
	h.base.send(ctx, u.VKID, sb.String(), keyboards.MainMenu())
}

func (h *UserHandler) handleProfile(ctx context.Context, u *models.User) {
	payments, _ := h.paymentSvc.ListByUser(ctx, u.ID)
	text := fmt.Sprintf(
		"РџСЂРѕС„РёР»СЊ\n\nРРјСЏ: %s %s\nР‘Р°Р»Р°РЅСЃ: %.2f RUB\nР—Р°РїСЂРѕСЃРѕРІ: %d (Р»РёРјРёС‚: %d)\nР РµРіРёСЃС‚СЂР°С†РёСЏ: %s\nРџР»Р°С‚РµР¶РµР№: %d",
		u.FirstName, u.LastName, u.Balance, u.RequestCount, u.RequestLimit, u.CreatedAt.Format("02.01.2006"), len(payments),
	)
	h.base.send(ctx, u.VKID, text, keyboards.MainMenu())
}

func (h *UserHandler) loadQuestionnaireItems(ctx context.Context) []string {
	items := []string{}
	raw, _ := h.settingsRepo.Get(ctx, models.SettingQuestionnaireItems)
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &items)
	}
	if len(items) == 0 {
		return questionnaire
	}
	out := make([]string, 0, len(items))
	for _, q := range items {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	if len(out) == 0 {
		return questionnaire
	}
	return out
}

func (h *UserHandler) renderFAQ(ctx context.Context) string {
	raw, _ := h.settingsRepo.Get(ctx, models.SettingFAQItems)
	if strings.TrimSpace(raw) != "" {
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err == nil {
			out := make([]string, 0, len(items))
			for _, it := range items {
				it = strings.TrimSpace(it)
				if it != "" {
					out = append(out, it)
				}
			}
			if len(out) > 0 {
				var sb strings.Builder
				sb.WriteString("FAQ:\n\n")
				for i, q := range out {
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
				}
				return sb.String()
			}
		}
	}

	faq, _ := h.settingsRepo.Get(ctx, models.SettingFAQText)
	if strings.TrimSpace(faq) != "" {
		return faq
	}
	return "Р Р°Р·РґРµР» FAQ РїРѕРєР° РЅРµ Р·Р°РїРѕР»РЅРµРЅ."
}

func (h *UserHandler) loadPromptForState(ctx context.Context, state models.BotState) string {
	switch state {
	case models.StateMapChat:
		if prompt := h.loadPromptValue(ctx, models.SettingMapPrompt, "system_prompt_map.txt"); prompt != "" {
			return prompt
		}
	case models.StateMainChat:
		if prompt := h.loadPromptValue(ctx, models.SettingGamePrompt, "system_prompt_game.txt"); prompt != "" {
			return prompt
		}
	}

	if prompt := h.loadPromptValue(ctx, models.SettingSystemPrompt, "system_prompt.txt"); prompt != "" {
		return prompt
	}
	return ""
}

func (h *UserHandler) loadPromptValue(ctx context.Context, settingKey, fileName string) string {
	if raw, _ := h.settingsRepo.Get(ctx, settingKey); strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if data, err := os.ReadFile(fileName); err == nil && strings.TrimSpace(string(data)) != "" {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (h *UserHandler) dialogTypeForState(state models.BotState) models.DialogType {
	switch state {
	case models.StateMapChat:
		return models.DialogMap
	default:
		return models.DialogMain
	}
}
