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
	"Как вас зовут?",
	"Чем вы занимаетесь?",
	"Как вы узнали о нас?",
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
		welcome = "Добро пожаловать! Я ваш AI-помощник."
	}

	consentText, _ := h.settingsRepo.Get(ctx, models.SettingConsentText)
	if consentText == "" {
		consentText = "Для продолжения необходимо согласие на обработку персональных данных и получение рассылок."
	}

	h.base.send(ctx, u.VKID, welcome+"\n\n"+consentText, keyboards.ConsentKeyboard())
	_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateConsent)
}

func (h *UserHandler) handleConsent(ctx context.Context, u *models.User, cmd string) {
	switch cmd {
	case "consent_accept":
		_ = h.userSvc.SaveConsent(ctx, u.VKID, true, false)
		h.base.send(ctx, u.VKID, "Хотите получать полезные рассылки?", keyboards.MailingConsentKeyboard())
	case "mailing_yes":
		_ = h.userSvc.SaveConsent(ctx, u.VKID, true, true)
		h.startQuestionnaire(ctx, u)
	case "mailing_no":
		h.startQuestionnaire(ctx, u)
	case "consent_decline":
		h.base.send(ctx, u.VKID, "Без согласия пользоваться ботом нельзя.", keyboards.Empty())
	}
}

func (h *UserHandler) startQuestionnaire(ctx context.Context, u *models.User) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
		h.base.send(ctx, u.VKID, "Выберите режим: Игра или Карта.", keyboards.MainMenu())
		return
	}
	_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateQuestionnaire)
	h.base.send(ctx, u.VKID, "Небольшая анкета. "+questions[0], keyboards.Empty())
}

func (h *UserHandler) handleQuestionnaire(ctx context.Context, u *models.User, text string) {
	questions := h.loadQuestionnaireItems(ctx)
	if len(questions) == 0 {
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateNone)
		h.base.send(ctx, u.VKID, "Выберите режим: Игра или Карта.", keyboards.MainMenu())
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
	h.base.send(ctx, u.VKID, "Анкета заполнена. Теперь можно выбрать режим.", keyboards.MainMenu())
}

func (h *UserHandler) handleMainState(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "game_chat":
		h.enterScenarioMode(ctx, u, models.StateMainChat)
	case "map_chat":
		h.enterScenarioMode(ctx, u, models.StateMapChat)
	case "support":
		_ = h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID, "Чат технической поддержки. Опишите проблему.", keyboards.BackOnly())
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
		h.base.send(ctx, u.VKID, "Выберите режим: Игра или Карта.", keyboards.MainMenu())
	}
}

func (h *UserHandler) enterScenarioMode(ctx context.Context, u *models.User, state models.BotState) {
	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogMain)
	if err != nil {
		slog.Error("get dialog for mode", "err", err)
		h.base.send(ctx, u.VKID, "Не удалось открыть режим. Попробуйте позже.", keyboards.MainMenu())
		return
	}
	_ = h.dialogRepo.ClearHistory(ctx, dialog.ID)
	_ = h.userSvc.UpdateState(ctx, u.VKID, state)

	reply, err := h.generateModeIntro(ctx, state)
	if err != nil {
		slog.Error("generate mode intro", "state", state, "err", err)
		h.base.send(ctx, u.VKID, "Не удалось запустить режим. Проверьте промпт.", keyboards.MainMenu())
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

	startMessage := "Начни диалог по правилам этого режима."
	if state == models.StateMainChat {
		startMessage = "Начни симуляцию по правилам режима. Сначала запроси необходимые данные и затем дай первый ход среды."
	}
	if state == models.StateMapChat {
		startMessage = "Начни режим карты. Сначала запроси исходные данные, нужные для чтения карты, и веди диалог в рамках инструкции."
	}

	messages := []models.AIMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: startMessage},
	}

	h.mon.RecordAICall()
	return h.aiSvc.Complete(ctx, messages)
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
		h.base.send(ctx, u.VKID, "Вы достигли лимита запросов. Пополните баланс или дождитесь сброса лимита.", keyboards.MainMenu())
		return
	}

	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogMain)
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

	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "Ошибка AI. Попробуйте позже.", keyboards.MainMenu())
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
		h.base.send(ctx, u.VKID, "Вы вернулись в меню.", keyboards.MainMenu())
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
	h.base.send(ctx, u.VKID, "Сообщение отправлено в поддержку. Мы ответим позже.", keyboards.BackOnly())
}

func (h *UserHandler) handlePaymentMenu(ctx context.Context, u *models.User) {
	info := fmt.Sprintf("Ваш баланс: %.2f RUB\n\nВыберите способ пополнения:", u.Balance)
	h.base.send(ctx, u.VKID, info, keyboards.PaymentMethods())
}

func (h *UserHandler) initiatePayment(ctx context.Context, u *models.User) {
	p, err := h.paymentSvc.CreatePayment(ctx, u.ID, 100, "Пополнение баланса")
	if err != nil {
		h.base.send(ctx, u.VKID, "Ошибка при создании платежа.", keyboards.MainMenu())
		return
	}
	h.base.send(ctx, u.VKID, fmt.Sprintf("Ссылка для оплаты:\n%s", p.ConfirmationURL), keyboards.MainMenu())
}

func (h *UserHandler) handleWalletPayment(ctx context.Context, u *models.User) {
	h.base.send(ctx, u.VKID, fmt.Sprintf("Баланс кошелька: %.2f RUB\n\nФункция в разработке.", u.Balance), keyboards.MainMenu())
}

func (h *UserHandler) handleServices(ctx context.Context, u *models.User) {
	products, err := h.paymentSvc.ListProducts(ctx)
	if err != nil || len(products) == 0 {
		h.base.send(ctx, u.VKID, "Каталог услуг пока пуст.", keyboards.MainMenu())
		return
	}
	var sb strings.Builder
	sb.WriteString("Услуги и цены:\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("- %s — %.0f RUB\n  %s\n\n", p.Name, p.Price, p.Description))
	}
	h.base.send(ctx, u.VKID, sb.String(), keyboards.MainMenu())
}

func (h *UserHandler) handleProfile(ctx context.Context, u *models.User) {
	payments, _ := h.paymentSvc.ListByUser(ctx, u.ID)
	text := fmt.Sprintf(
		"Профиль\n\nИмя: %s %s\nБаланс: %.2f RUB\nЗапросов: %d (лимит: %d)\nРегистрация: %s\nПлатежей: %d",
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
	return "Раздел FAQ пока не заполнен."
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
