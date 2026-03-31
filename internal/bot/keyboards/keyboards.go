// Package keyboards содержит VK-клавиатуры для разных экранов бота.
package keyboards

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VK Keyboard API структуры
type Keyboard struct {
	OneTime bool       `json:"one_time"`
	Inline  bool       `json:"inline"`
	Buttons [][]Button `json:"buttons"`
}

type Button struct {
	Action Action `json:"action"`
	Color  string `json:"color,omitempty"` // primary | secondary | negative | positive
}

type Action struct {
	Type    string `json:"type"` // text | callback | open_link
	Label   string `json:"label,omitempty"`
	Payload string `json:"payload,omitempty"`
	Link    string `json:"link,omitempty"`
}

// Serialize возвращает JSON строку клавиатуры для VK API
func (k *Keyboard) Serialize() string {
	b, _ := json.Marshal(k)
	return string(b)
}

// Empty — пустая клавиатура (скрыть)
func Empty() *Keyboard {
	return &Keyboard{Buttons: [][]Button{}}
}

// MainMenu — основное меню пользователя
func MainMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("💬 Главный диалог", "positive", `{"cmd":"main_chat"}`),
				btn("🛠 Поддержка", "secondary", `{"cmd":"support"}`),
			},
			{
				btn("💳 Пополнить", "primary", `{"cmd":"payment"}`),
				btn("🛍 Услуги", "primary", `{"cmd":"services"}`),
			},
			{
				btn("📊 Мой профиль", "secondary", `{"cmd":"profile"}`),
				btn("❓ FAQ", "secondary", `{"cmd":"faq"}`),
			},
		},
	}
}

// ConsentKeyboard — согласие на обработку данных
func ConsentKeyboard() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("✅ Принять", "positive", `{"cmd":"consent_accept"}`),
				btn("❌ Отказать", "negative", `{"cmd":"consent_decline"}`),
			},
		},
	}
}

// MailingConsentKeyboard — согласие на рассылку
func MailingConsentKeyboard() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("✅ Да, хочу", "positive", `{"cmd":"mailing_yes"}`),
				btn("❌ Нет", "secondary", `{"cmd":"mailing_no"}`),
			},
		},
	}
}

// PaymentMethods — выбор способа оплаты
func PaymentMethods() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("💳 Карта / СБП", "primary", `{"cmd":"pay_card"}`),
			},
			{
				btn("💼 Внутренний кошелёк", "secondary", `{"cmd":"pay_wallet"}`),
			},
			{
				btn("← Назад", "secondary", `{"cmd":"back"}`),
			},
		},
	}
}

// AdminMenu — меню администратора
func AdminMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("� AI Диалог", "positive", `{"cmd":"main_chat"}`),
				btn("🛠 Поддержка", "secondary", `{"cmd":"support"}`),
			},
			{
				btn("👥 Пользователи", "primary", `{"cmd":"admin_users"}`),
				btn("🔗 Ссылки", "primary", `{"cmd":"admin_invites"}`),
			},
			{
				btn("⚙️ Настройки", "secondary", `{"cmd":"admin_settings"}`),
				btn("📊 Мониторинг", "secondary", `{"cmd":"admin_metrics"}`),
			},
			{
				btn("📋 Модераторы", "secondary", `{"cmd":"admin_mods"}`),
				btn("📝 Аудит", "secondary", `{"cmd":"admin_audit"}`),
			},
		},
	}
}

// AdminChatMenu — меню когда админ в режиме диалога с AI
func AdminChatMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("👑 Панель админа", "primary", `{"cmd":"admin_panel"}`),
				btn("🛠 Поддержка", "secondary", `{"cmd":"support"}`),
			},
		},
	}
}

// ModeratorMenu — меню модератора
func ModeratorMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("💬 Поддержка", "primary", `{"cmd":"mod_support"}`),
				btn("🔗 Создать ссылку", "secondary", `{"cmd":"mod_invite"}`),
			},
			{
				btn("👤 Проверить пользователя", "secondary", `{"cmd":"mod_check_user"}`),
			},
		},
	}
}

// YesNo — универсальные кнопки подтверждения
func YesNo(yesPayload, noPayload string) *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("✅ Да", "positive", yesPayload),
				btn("❌ Нет", "negative", noPayload),
			},
		},
	}
}

// BackOnly — только кнопка «Назад»
func BackOnly() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{btn("← Назад", "secondary", `{"cmd":"back"}`)},
		},
	}
}

// AccessRequestActions — кнопки одобрить/отклонить заявку на вступление
// requestID передаётся в payload чтобы знать какую заявку обрабатывать
func AccessRequestActions(requestID int64, vkID int64, name string) *Keyboard {
	approvePayload := fmt.Sprintf(`{"cmd":"approve_request","req_id":%d,"vk_id":%d}`, requestID, vkID)
	rejectPayload := fmt.Sprintf(`{"cmd":"reject_request","req_id":%d,"vk_id":%d}`, requestID, vkID)
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = fmt.Sprintf("id%d", vkID)
	}
	if len(displayName) > 18 {
		displayName = displayName[:15] + "..."
	}
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("✅ "+displayName, "positive", approvePayload),
				btn("❌ "+displayName, "negative", rejectPayload),
			},
		},
	}
}

// RequestAccess — кнопка подачи заявки для гостя
func RequestAccess() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{btn("📩 Подать заявку", "primary", `{"cmd":"request_access"}`)},
		},
	}
}

// UserListInline — inline-клавиатура со списком пользователей (по 2 в ряд, до 10)
// Каждая кнопка открывает карточку пользователя
func UserListInline(users []UserButton, offset int, total int) *Keyboard {
	kb := &Keyboard{Inline: true}
	row := []Button{}
	for i, u := range users {
		label := u.Name
		if len(label) > 20 {
			label = label[:18] + "…"
		}
		payload := fmt.Sprintf(`{"cmd":"admin_user_detail","vk_id":%d}`, u.VKID)
		row = append(row, btn(label, "secondary", payload))
		if len(row) == 2 || i == len(users)-1 {
			kb.Buttons = append(kb.Buttons, row)
			row = []Button{}
		}
	}
	// Пагинация
	nav := []Button{}
	if offset > 0 {
		nav = append(nav, btn("◀️ Назад", "secondary",
			fmt.Sprintf(`{"cmd":"admin_users_page","offset":%d}`, offset-8)))
	}
	if offset+len(users) < total {
		nav = append(nav, btn("▶️ Далее", "secondary",
			fmt.Sprintf(`{"cmd":"admin_users_page","offset":%d}`, offset+8)))
	}
	if len(nav) > 0 {
		kb.Buttons = append(kb.Buttons, nav)
	}
	return kb
}

// UserButton — данные для кнопки пользователя
type UserButton struct {
	VKID int64
	Name string
}

// UserActionsInline — inline-кнопки действий над конкретным пользователем
func UserActionsInline(vkID int64) *Keyboard {
	return &Keyboard{
		Inline: true,
		Buttons: [][]Button{
			{
				btn("🚫 Бан", "negative", fmt.Sprintf(`{"cmd":"admin_ban","vk_id":%d}`, vkID)),
				btn("❄️ Охладить 1ч", "secondary", fmt.Sprintf(`{"cmd":"admin_cool","vk_id":%d,"mins":60}`, vkID)),
			},
			{
				btn("✅ Разбанить", "positive", fmt.Sprintf(`{"cmd":"admin_unban","vk_id":%d}`, vkID)),
				btn("👮 Модератор", "primary", fmt.Sprintf(`{"cmd":"admin_set_mod","vk_id":%d}`, vkID)),
			},
			{
				btn("👤 Разжаловать", "secondary", fmt.Sprintf(`{"cmd":"admin_set_user","vk_id":%d}`, vkID)),
				btn("📊 Лимит запросов", "secondary", fmt.Sprintf(`{"cmd":"admin_set_limit","vk_id":%d}`, vkID)),
			},
		},
	}
}

func btn(label, color, payload string) Button {
	return Button{
		Color: color,
		Action: Action{
			Type:    "text",
			Label:   label,
			Payload: payload,
		},
	}
}

// MakeBtn — публичная версия btn для использования в handlers
func MakeBtn(label, color, payload string) Button {
	return btn(label, color, payload)
}
