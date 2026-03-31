package handlers

import (
	"context"
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

// Анкета — вопросы по порядку
var questionnaire = []string{
	"Как вас зовут?",
	"Чем вы занимаетесь?",
	"Как вы узнали о нас?",
}

// UserHandler — обработчик сообщений обычного пользователя
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
		welcome = "👋 Добро пожаловать! Я ваш AI-помощник."
	}

	consentText, _ := h.settingsRepo.Get(ctx, models.SettingConsentText)
	if consentText == "" {
		consentText = "📋 Для продолжения необходимо согласие на обработку персональных данных и получение рассылок."
	}

	h.base.send(ctx, u.VKID, welcome+"\n\n"+consentText, keyboards.ConsentKeyboard())
	h.userSvc.UpdateState(ctx, u.VKID, models.StateConsent)
}

func (h *UserHandler) handleConsent(ctx context.Context, u *models.User, cmd string) {
	switch cmd {
	case "consent_accept":
		h.userSvc.SaveConsent(ctx, u.VKID, true, false)
		h.base.send(ctx, u.VKID, "Хотите получать полезные рассылки от нас?", keyboards.MailingConsentKeyboard())
		// Переходим к выбору рассылки (промежуточный шаг)
	case "mailing_yes":
		h.userSvc.SaveConsent(ctx, u.VKID, true, true)
		h.startQuestionnaire(ctx, u)
	case "mailing_no":
		h.startQuestionnaire(ctx, u)
	case "consent_decline":
		h.base.send(ctx, u.VKID,
			"❌ Без согласия на обработку данных пользование ботом недоступно. Вы можете вернуться позже.",
			keyboards.Empty())
	}
}

func (h *UserHandler) startQuestionnaire(ctx context.Context, u *models.User) {
	h.userSvc.UpdateState(ctx, u.VKID, models.StateQuestionnaire)
	h.base.send(ctx, u.VKID, "📝 Небольшая анкета. "+questionnaire[0], keyboards.Empty())
}

func (h *UserHandler) handleQuestionnaire(ctx context.Context, u *models.User, text string) {
	answers, _ := h.userSvc.GetQAnswers(ctx, u.ID)
	currentQ := len(answers)

	if currentQ < len(questionnaire) {
		h.userSvc.SaveQAnswer(ctx, u.ID, questionnaire[currentQ], text)
		currentQ++
	}

	if currentQ < len(questionnaire) {
		h.base.send(ctx, u.VKID, questionnaire[currentQ], keyboards.Empty())
		return
	}

	// Анкета завершена
	h.userSvc.UpdateState(ctx, u.VKID, models.StateMainChat)
	h.base.send(ctx, u.VKID,
		"✅ Анкета заполнена! Чем могу помочь?",
		keyboards.MainMenu())
}

func (h *UserHandler) handleMainState(ctx context.Context, u *models.User, msg object.MessagesMessage, cmd, text string) {
	switch cmd {
	case "support":
		h.userSvc.UpdateState(ctx, u.VKID, models.StateSupport)
		h.base.send(ctx, u.VKID,
			"🛠 Чат технической поддержки. Опишите проблему.",
			keyboards.BackOnly())

	case "payment":
		h.handlePaymentMenu(ctx, u)

	case "services":
		h.handleServices(ctx, u)

	case "profile":
		h.handleProfile(ctx, u)

	case "faq":
		faq, _ := h.settingsRepo.Get(ctx, models.SettingFAQText)
		if faq == "" {
			faq = "❓ Раздел FAQ пока не заполнен."
		}
		h.base.send(ctx, u.VKID, faq, keyboards.MainMenu())

	case "pay_card":
		h.initiatePayment(ctx, u)

	case "pay_wallet":
		h.handleWalletPayment(ctx, u)

	default:
		// Основной диалог с AI
		h.handleAIChat(ctx, u, text)
	}
}

func (h *UserHandler) handleAIChat(ctx context.Context, u *models.User, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	// Проверка лимита
	ok, err := h.userSvc.CheckLimit(ctx, u)
	if err != nil {
		slog.Error("check limit", "err", err)
	}
	if !ok {
		h.base.send(ctx, u.VKID,
			"⚠️ Вы достигли лимита запросов. Пополните баланс или дождитесь сброса лимита.",
			keyboards.MainMenu())
		return
	}

	// Получаем диалог
	dialog, err := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogMain)
	if err != nil {
		slog.Error("get dialog", "err", err)
		h.mon.RecordError()
		return
	}

	// Сохраняем сообщение пользователя
	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})

	// Получаем историю для AI (последние 20 сообщений)
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

	// Запрос к AI
	h.mon.RecordAICall()
	reply, err := h.aiSvc.Complete(ctx, aiMessages)
	if err != nil {
		slog.Error("ai complete", "err", err)
		h.mon.RecordError()
		h.base.send(ctx, u.VKID, "⚠️ Ошибка AI. Попробуйте позже.", keyboards.MainMenu())
		return
	}

	// Сохраняем ответ AI
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
		h.base.send(ctx, u.VKID, "↩️ Вы вернулись в основной диалог.", keyboards.MainMenu())
		return
	}

	// Сохраняем в отдельный диалог поддержки
	dialog, _ := h.dialogRepo.GetOrCreateDialog(ctx, u.ID, models.DialogSupport)
	h.dialogRepo.SaveMessage(ctx, &models.Message{
		DialogID: dialog.ID,
		UserID:   u.ID,
		Role:     models.MessageRoleUser,
		Type:     models.MessageTypeText,
		Content:  text,
	})
	h.base.send(ctx, u.VKID,
		"✅ Сообщение отправлено в поддержку. Мы ответим в ближайшее время.",
		keyboards.BackOnly())
}

func (h *UserHandler) handlePaymentMenu(ctx context.Context, u *models.User) {
	info := fmt.Sprintf("💳 Ваш баланс: %.2f ₽\n\nВыберите способ пополнения:", u.Balance)
	h.base.send(ctx, u.VKID, info, keyboards.PaymentMethods())
}

func (h *UserHandler) initiatePayment(ctx context.Context, u *models.User) {
	// Минимальное пополнение 100 руб
	p, err := h.paymentSvc.CreatePayment(ctx, u.ID, 100, "Пополнение баланса")
	if err != nil {
		h.base.send(ctx, u.VKID, "❌ Ошибка при создании платежа.", keyboards.MainMenu())
		return
	}
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("💳 Ссылка для оплаты:\n%s\n\nПосле оплаты баланс пополнится автоматически.", p.ConfirmationURL),
		keyboards.MainMenu())
}

func (h *UserHandler) handleWalletPayment(ctx context.Context, u *models.User) {
	h.base.send(ctx, u.VKID,
		fmt.Sprintf("💼 Баланс кошелька: %.2f ₽\n\nФункция оплаты с кошелька в разработке.", u.Balance),
		keyboards.MainMenu())
}

func (h *UserHandler) handleServices(ctx context.Context, u *models.User) {
	products, err := h.paymentSvc.ListProducts(ctx)
	if err != nil || len(products) == 0 {
		h.base.send(ctx, u.VKID, "🛍 Каталог услуг пока пуст.", keyboards.MainMenu())
		return
	}
	var sb strings.Builder
	sb.WriteString("🛍 Услуги и цены:\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("• %s — %.0f ₽\n  %s\n\n", p.Name, p.Price, p.Description))
	}
	h.base.send(ctx, u.VKID, sb.String(), keyboards.MainMenu())
}

func (h *UserHandler) handleProfile(ctx context.Context, u *models.User) {
	payments, _ := h.paymentSvc.ListByUser(ctx, u.ID)
	text := fmt.Sprintf(
		"👤 Профиль\n\n"+
			"Имя: %s %s\n"+
			"Баланс: %.2f ₽\n"+
			"Запросов: %d (лимит: %d)\n"+
			"Регистрация: %s\n"+
			"Платежей: %d",
		u.FirstName, u.LastName,
		u.Balance,
		u.RequestCount, u.RequestLimit,
		u.CreatedAt.Format("02.01.2006"),
		len(payments),
	)
	h.base.send(ctx, u.VKID, text, keyboards.MainMenu())
}
