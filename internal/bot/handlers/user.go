package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

// РђРЅРєРµС‚Р° вЂ” РІРѕРїСЂРѕСЃС‹ РїРѕ РїРѕСЂСЏРґРєСѓ
var questionnaire = []string{
	"РљР°Рє РІР°СЃ Р·РѕРІСѓС‚?",
	"Р§РµРј РІС‹ Р·Р°РЅРёРјР°РµС‚РµСЃСЊ?",
	"РљР°Рє РІС‹ СѓР·РЅР°Р»Рё Рѕ РЅР°СЃ?",
}

// UserHandler вЂ” РѕР±СЂР°Р±РѕС‚С‡РёРє СЃРѕРѕР±С‰РµРЅРёР№ РѕР±С‹С‡РЅРѕРіРѕ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ
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
		welcome = "рџ‘‹ Р”РѕР±СЂРѕ РїРѕР¶Р°Р»РѕРІР°С‚СЊ! РЇ РІР°С€ AI-РїРѕРјРѕС‰РЅРёРє."
	}

	consentText, _ := h.settingsRepo.Get(ctx, models.SettingConsentText)
	if consentText == "" {
		consentText = "рџ“‹ Р”Р»СЏ РїСЂРѕРґРѕР»Р¶РµРЅРёСЏ РЅРµРѕР±С…РѕРґРёРјРѕ СЃРѕРіР»Р°СЃРёРµ РЅР° РѕР±СЂР°Р±РѕС‚РєСѓ РїРµСЂСЃРѕРЅР°Р»СЊРЅС‹С… РґР°РЅРЅС‹С… Рё РїРѕР»СѓС‡РµРЅРёРµ СЂР°СЃСЃС‹Р»РѕРє."
	}

	h.base.send(ctx, u.VKID, welcome+"\n\n"+consentText, keyboards.ConsentKeyboard())
	h.userSvc.UpdateState(ctx, u.VKID, models.StateConsent)
}

func (h *UserHandler) handleConsent(ctx context.Context, u *models.User, cmd string) {
	switch cmd {
	case "consent_accept":
		h.userSvc.SaveConsent(ctx, u.VKID, true, false)
		h.base.send(ctx, u.VKID, "РҐРѕС‚РёС‚Рµ РїРѕР»СѓС‡Р°С‚СЊ РїРѕР»РµР·РЅС‹Рµ СЂР°СЃСЃС‹Р»РєРё РѕС‚ РЅР°СЃ?", keyboards.MailingConsentKeyboard())
		// РџРµСЂРµС…РѕРґРёРј Рє РІС‹Р±РѕСЂСѓ СЂР°СЃСЃС‹Р»РєРё (РїСЂРѕРјРµР¶СѓС‚РѕС‡РЅС‹Р№ С€Р°Рі)
	case "mailing_yes":
		h.userSvc.SaveConsent(ctx, u.VKID, true, true)
		h.startQuestionnaire(ctx, u)
	case "mailing_no":
		h.startQuestionnaire(ctx, u)
	case "consent_decline":
		h.base.send(ctx, u.VKID,
			"вќЊ Р‘РµР· СЃРѕРіР»Р°СЃРёСЏ РЅР° РѕР±СЂР°Р±РѕС‚РєСѓ РґР°РЅРЅС‹С… РїРѕР»СЊР·РѕРІР°РЅРёРµ Р±РѕС‚РѕРј РЅРµРґРѕСЃС‚СѓРїРЅРѕ. Р’С‹ РјРѕР¶РµС‚Рµ РІРµСЂРЅСѓС‚СЊСЃСЏ РїРѕР·Р¶Рµ.",
			keyboards.Empty())
	}
}

func (h *UserHandler) startQuestionnaire(ctx context.Context, u *models.User) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
		h.base.send(ctx, u.VKID, "✅ Можете сразу начать диалог.", keyboards.MainMenu())
		return
	}
	h.userSvc.UpdateState(ctx, u.VKID, models.StateQuestionnaire)
	h.base.send(ctx, u.VKID, "📝 Небольшая анкета. "+questions[0], keyboards.Empty())
}

func (h *UserHandler) handleQuestionnaire(ctx context.Context, u *models.User, text string) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
		h.base.send(ctx, u.VKID, "✅ Можете сразу начать диалог.", keyboards.MainMenu())
		return
	}

	answers, _ := h.userSvc.GetQAnswers(ctx, u.ID)
	currentQ := len(answers)

	if currentQ < len(questions) {
		h.userSvc.SaveQAnswer(ctx, u.ID, questions[currentQ], text)
		currentQ++
	}

	if currentQ < len(questions) {
		h.base.send(ctx, u.VKID, questions[currentQ], keyboards.Empty())
		return
	}

	h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
	h.base.send(ctx, u.VKID, "✅ Анкета заполнена! Чем могу помочь?", keyboards.MainMenu())
}
func (h *UserHandler) handleMainState(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "support":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID,
			"рџ›  Р§Р°С‚ С‚РµС…РЅРёС‡РµСЃРєРѕР№ РїРѕРґРґРµСЂР¶РєРё. РћРїРёС€РёС‚Рµ РїСЂРѕР±Р»РµРјСѓ.",
			keyboards.BackOnly())

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
		// РћСЃРЅРѕРІРЅРѕР№ РґРёР°Р»РѕРі СЃ AI
		h.handleAIChat(ctx, u, text)
	}
}

func (h *UserHandler) handleAIChat(ctx context.Context, u *models.User, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	// РџСЂРѕРІРµСЂРєР° Р»РёРјРёС‚Р°
	ok, err := h.userSvc.CheckLimit(ctx, u)
	if err != nil {
		slog.Error("check limit", "err", err)
	}
	if !ok {
		h.base.send(ctx, u.VKID,
			"вљ пёЏ Р’С‹ РґРѕСЃС‚РёРіР»Рё Р»РёРјРёС‚Р° Р·Р°РїСЂРѕСЃРѕРІ. РџРѕРїРѕР»РЅРёС‚Рµ Р±Р°Р»Р°РЅСЃ РёР»Рё РґРѕР¶РґРёС‚РµСЃСЊ СЃР±СЂРѕСЃР° Р»РёРјРёС‚Р°.",
			keyboards.MainMenu())
		return
	}

	// РџРѕР»СѓС‡Р°РµРј РґРёР°Р»РѕРі
	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogMain)
	if err != nil {
		slog.Error("get dialog", "err", err)
		h.mon.RecordError()
		return
	}

	// РЎРѕС…СЂР°РЅСЏРµРј СЃРѕРѕР±С‰РµРЅРёРµ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ
	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})

	// РџРѕР»СѓС‡Р°РµРј РёСЃС‚РѕСЂРёСЋ РґР»СЏ AI (РїРѕСЃР»РµРґРЅРёРµ 20 СЃРѕРѕР±С‰РµРЅРёР№)
	history, err := h.dialogRepo.GetHistory(ctx, dialog.ID, 20)
	if err != nil {
		slog.Error("get history", "err", err)
	}

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

	// Р—Р°РїСЂРѕСЃ Рє AI
	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "вљ пёЏ РћС€РёР±РєР° AI. РџРѕРїСЂРѕР±СѓР№С‚Рµ РїРѕР·Р¶Рµ.", keyboards.MainMenu())
		return
	}

	// РЎРѕС…СЂР°РЅСЏРµРј РѕС‚РІРµС‚ AI
	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleAssistant,
		Type:     models.MessageTypeText,
		Content:  reply,
	})

	h.userSvc.IncrRequestCount(ctx, u.VKID)
	h.base.send(ctx, u.VKID, reply, keyboards.MainMenu())
}

func (h *UserHandler) handleSupportDialog(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	if cmd == "back" {
		h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
		h.base.send(ctx, u.VKID, "в†©пёЏ Р’С‹ РІРµСЂРЅСѓР»РёСЃСЊ РІ РѕСЃРЅРѕРІРЅРѕР№ РґРёР°Р»РѕРі.", keyboards.MainMenu())
		return
	}

	// РЎРѕС…СЂР°РЅСЏРµРј РІ РѕС‚РґРµР»СЊРЅС‹Р№ РґРёР°Р»РѕРі РїРѕРґРґРµСЂР¶РєРё
	dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})
	h.base.send(ctx, u.VKID,
		"вњ… РЎРѕРѕР±С‰РµРЅРёРµ РѕС‚РїСЂР°РІР»РµРЅРѕ РІ РїРѕРґРґРµСЂР¶РєСѓ. РњС‹ РѕС‚РІРµС‚РёРј РІ Р±Р»РёР¶Р°Р№С€РµРµ РІСЂРµРјСЏ.",
		keyboards.BackOnly())
}

func (h *UserHandler) handlePaymentMenu(ctx context.Context, u *models.User) {
	info := fmt.Sprintf("рџ’і Р’Р°С€ Р±Р°Р»Р°РЅСЃ: %.2f в‚Ѕ\n\nР’С‹Р±РµСЂРёС‚Рµ СЃРїРѕСЃРѕР± РїРѕРїРѕР»РЅРµРЅРёСЏ:", u.Balance)
	h.base.send(ctx, u.VKID, info, keyboards.PaymentMethods())
}

func (h *UserHandler) initiatePayment(ctx context.Context, u *models.User) {
	// РњРёРЅРёРјР°Р»СЊРЅРѕРµ РїРѕРїРѕР»РЅРµРЅРёРµ 100 СЂСѓР±
	p, err := h.paymentSvc.CreatePayment(ctx, u.ID, 100, "РџРѕРїРѕР»РЅРµРЅРёРµ Р±Р°Р»Р°РЅСЃР°")
	if err != nil {
		h.base.send(ctx, u.VKID, "вќЊ РћС€РёР±РєР° РїСЂРё СЃРѕР·РґР°РЅРёРё РїР»Р°С‚РµР¶Р°.", keyboards.MainMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("рџ’і РЎСЃС‹Р»РєР° РґР»СЏ РѕРїР»Р°С‚С‹:\n%s\n\nРџРѕСЃР»Рµ РѕРїР»Р°С‚С‹ Р±Р°Р»Р°РЅСЃ РїРѕРїРѕР»РЅРёС‚СЃСЏ Р°РІС‚РѕРјР°С‚РёС‡РµСЃРєРё.", p.ConfirmationURL),
		keyboards.MainMenu())
}

func (h *UserHandler) handleWalletPayment(ctx context.Context, u *models.User) {
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("рџ’ј Р‘Р°Р»Р°РЅСЃ РєРѕС€РµР»СЊРєР°: %.2f в‚Ѕ\n\nР¤СѓРЅРєС†РёСЏ РѕРїР»Р°С‚С‹ СЃ РєРѕС€РµР»СЊРєР° РІ СЂР°Р·СЂР°Р±РѕС‚РєРµ.", u.Balance),
		keyboards.MainMenu())
}

func (h *UserHandler) handleServices(ctx context.Context, u *models.User) {
	products, err := h.paymentSvc.ListProducts(ctx)
	if err != nil || len(products) == 0 {
		h.base.send(ctx, u.VKID, "рџ›Ќ РљР°С‚Р°Р»РѕРі СѓСЃР»СѓРі РїРѕРєР° РїСѓСЃС‚.", keyboards.MainMenu())
		return
	}
	var sb strings.Builder
	sb.WriteString("рџ›Ќ РЈСЃР»СѓРіРё Рё С†РµРЅС‹:\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("вЂў %s вЂ” %.0f в‚Ѕ\n  %s\n\n", p.Name, p.Price, p.Description))
	}
	h.base.send(ctx, u.VKID, sb.String(), keyboards.MainMenu())
}

func (h *UserHandler) handleProfile(ctx context.Context, u *models.User) {
	payments, _ := h.paymentSvc.ListByUser(ctx, u.ID)
	text := fmt.Sprintf(
		"рџ‘¤ РџСЂРѕС„РёР»СЊ\n\n"+
			"РРјСЏ: %s %s\n"+
			"Р‘Р°Р»Р°РЅСЃ: %.2f в‚Ѕ\n"+
			"Р—Р°РїСЂРѕСЃРѕРІ: %d (Р»РёРјРёС‚: %d)\n"+
			"Р РµРіРёСЃС‚СЂР°С†РёСЏ: %s\n"+
			"РџР»Р°С‚РµР¶РµР№: %d",
		u.FirstName, u.LastName,
		u.Balance,
		u.RequestCount, u.RequestLimit,
		u.CreatedAt.Format("02.01.2006"),
		len(payments),
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
				sb.WriteString("❓ FAQ:\n\n")
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
	return "❓ Раздел FAQ пока не заполнен."
}
