package handlers

import (
	"context"
	"strings"

	"github.com/vitaly06/ai-vk-bot/internal/models"
	aiSvc "github.com/vitaly06/ai-vk-bot/internal/services/ai"
)

const scenarioOutputGuard = `Это пользовательский чат.
Отвечай только обычным текстом для пользователя.
Нельзя выводить JSON, YAML, XML, markdown-схемы, поля node_*, route, input_type, intensity, confidence, служебные состояния, внутренние шаги анализа и технические сводки.
Команду "Режим отладки" выполняй только если пользователь написал ровно: Режим отладки
Во всех остальных случаях отвечай только по сцене или по смыслу режима.
Если пользователь написал короткое приветствие, короткий звук, одно слово или очень короткую реплику вроде "ку", "привет", "го", "начать", "дальше", это не техкоманда: продолжай нормальный диалог режима и дай содержательный ответ.`

const scenarioRepairPrompt = `Переформулируй предыдущий ответ в нормальный пользовательский формат.
Нужен только финальный ответ для человека.
Запрещено выводить JSON, списки полей, node_*, route, input_type, скрытую диагностику и любые внутренние структуры.
Если это не режим отладки, ответ должен быть обычным текстом сцены или обычным ответом режима.`

func appendScenarioGuard(messages []models.AIMessage) []models.AIMessage {
	out := make([]models.AIMessage, 0, len(messages)+1)
	out = append(out, messages...)
	out = append(out, models.AIMessage{Role: "system", Content: scenarioOutputGuard})
	return out
}

func repairScenarioReply(ctx context.Context, svc *aiSvc.Service, messages []models.AIMessage, userText, reply string) (string, error) {
	if isDebugCommand(userText) || !looksLikeStructuredReply(reply) {
		return reply, nil
	}

	retryMessages := make([]models.AIMessage, 0, len(messages)+2)
	retryMessages = append(retryMessages, messages...)
	retryMessages = append(retryMessages,
		models.AIMessage{Role: "system", Content: scenarioRepairPrompt},
		models.AIMessage{Role: "assistant", Content: reply},
	)

	fixed, err := svc.Complete(ctx, retryMessages)
	if err != nil {
		return "", err
	}
	if looksLikeStructuredReply(fixed) {
		return "Продолжим без технической сводки. Опишите действие, выбор или состояние одним сообщением, и я отвечу по сцене.", nil
	}
	return fixed, nil
}

func isDebugCommand(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), "Режим отладки")
}

func looksLikeStructuredReply(reply string) bool {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "```json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return true
	}

	signals := []string{
		`"node_`,
		`"route"`,
		`"input_type"`,
		`"safety_precheck"`,
		`"dominant_`,
		`"intensity"`,
		`"confidence"`,
		`"novelty"`,
		`"active_barrier"`,
		`"safe_point_available"`,
		`init_story`,
	}
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			return true
		}
	}

	return false
}
