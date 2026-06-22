package handlers

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type FAQEntry struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func defaultFAQEntries() []FAQEntry {
	return []FAQEntry{
		{
			Question: "Как начать пользоваться ботом?",
			Answer:   "Откройте режим «Зеркало» или «Карта», после чего просто напишите первое сообщение. Если включена стартовая анкета, сначала ответьте на её вопросы.",
		},
		{
			Question: "Как пополнить баланс?",
			Answer:   "Нажмите кнопку «Пополнить» в меню бота и выберите подходящий способ оплаты.",
		},
	}
}

func cloneFAQEntries(items []FAQEntry) []FAQEntry {
	out := make([]FAQEntry, 0, len(items))
	for _, item := range items {
		q := strings.TrimSpace(item.Question)
		a := strings.TrimSpace(item.Answer)
		if q == "" {
			continue
		}
		out = append(out, FAQEntry{
			Question: q,
			Answer:   a,
		})
	}
	return out
}

func parseFAQEntries(raw string, fallback []FAQEntry) []FAQEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cloneFAQEntries(fallback)
	}

	var entries []FAQEntry
	if err := json.Unmarshal([]byte(raw), &entries); err == nil {
		if normalized := cloneFAQEntries(entries); len(normalized) > 0 {
			return normalized
		}
	}

	var questions []string
	if err := json.Unmarshal([]byte(raw), &questions); err == nil {
		entries = make([]FAQEntry, 0, len(questions))
		for _, q := range questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			entries = append(entries, FAQEntry{
				Question: q,
				Answer:   defaultFAQAnswer(q),
			})
		}
		if len(entries) > 0 {
			return entries
		}
	}

	return cloneFAQEntries(fallback)
}

func defaultFAQAnswer(question string) string {
	switch strings.TrimSpace(question) {
	case "Как начать пользоваться ботом?":
		return "Откройте режим «Зеркало» или «Карта», после чего просто напишите первое сообщение. Если включена стартовая анкета, сначала ответьте на её вопросы."
	case "Как пополнить баланс?":
		return "Нажмите кнопку «Пополнить» в меню бота и выберите подходящий способ оплаты."
	default:
		return ""
	}
}

func encodeStateValue(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeStateValue(value string) (string, bool) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}
	return string(data), true
}
