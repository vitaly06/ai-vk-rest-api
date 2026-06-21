package keyboards

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Keyboard struct {
	OneTime bool       `json:"one_time"`
	Inline  bool       `json:"inline"`
	Buttons [][]Button `json:"buttons"`
}

type Button struct {
	Action Action `json:"action"`
	Color  string `json:"color,omitempty"`
}

type Action struct {
	Type    string `json:"type"`
	Label   string `json:"label,omitempty"`
	Payload string `json:"payload,omitempty"`
	Link    string `json:"link,omitempty"`
}

func (k *Keyboard) Serialize() string {
	b, _ := json.Marshal(k)
	return string(b)
}

func Empty() *Keyboard {
	return &Keyboard{Buttons: [][]Button{}}
}

func MainMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("Р—РµСЂРєР°Р»Рѕ", "positive", `{"cmd":"game_chat"}`),
				btn("РљР°СЂС‚Р°", "primary", `{"cmd":"map_chat"}`),
			},
			{
				btn("РџРѕРґРґРµСЂР¶РєР°", "secondary", `{"cmd":"support"}`),
				btn("РџРѕРїРѕР»РЅРёС‚СЊ", "primary", `{"cmd":"payment"}`),
			},
			{
				btn("РЈСЃР»СѓРіРё", "primary", `{"cmd":"services"}`),
				btn("РњРѕР№ РїСЂРѕС„РёР»СЊ", "secondary", `{"cmd":"profile"}`),
			},
			{
				btn("FAQ", "secondary", `{"cmd":"faq"}`),
			},
		},
	}
}

func ConsentKeyboard() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("РџСЂРёРЅСЏС‚СЊ", "positive", `{"cmd":"consent_accept"}`),
				btn("РћС‚РєР°Р·Р°С‚СЊСЃСЏ", "negative", `{"cmd":"consent_decline"}`),
			},
		},
	}
}

func MailingConsentKeyboard() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("Р”Р°, С…РѕС‡Сѓ", "positive", `{"cmd":"mailing_yes"}`),
				btn("РќРµС‚", "secondary", `{"cmd":"mailing_no"}`),
			},
		},
	}
}

func PaymentMethods() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{btn("РљР°СЂС‚Р° / РЎР‘Рџ", "primary", `{"cmd":"pay_card"}`)},
			{btn("Р’РЅСѓС‚СЂРµРЅРЅРёР№ РєРѕС€РµР»РµРє", "secondary", `{"cmd":"pay_wallet"}`)},
			{btn("РќР°Р·Р°Рґ", "secondary", `{"cmd":"back"}`)},
		},
	}
}

func AdminMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("Р—РµСЂРєР°Р»Рѕ", "positive", `{"cmd":"main_chat"}`),
				btn("РљР°СЂС‚Р°", "positive", `{"cmd":"map_chat"}`),
			},
			{
				btn("РџРѕРґРґРµСЂР¶РєР°", "secondary", `{"cmd":"support"}`),
			},
			{
				btn("РџРѕР»СЊР·РѕРІР°С‚РµР»Рё", "primary", `{"cmd":"admin_users"}`),
				btn("РЎСЃС‹Р»РєРё", "primary", `{"cmd":"admin_invites"}`),
			},
			{
				btn("РќР°СЃС‚СЂРѕР№РєРё", "secondary", `{"cmd":"admin_settings"}`),
				btn("РњРѕРЅРёС‚РѕСЂРёРЅРі", "secondary", `{"cmd":"admin_metrics"}`),
			},
			{
				btn("РњРѕРґРµСЂР°С‚РѕСЂС‹", "secondary", `{"cmd":"admin_mods"}`),
				btn("РђСѓРґРёС‚", "secondary", `{"cmd":"admin_audit"}`),
			},
		},
	}
}

func AdminSettingsEditorMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("РџСЂРёРІРµС‚СЃС‚РІРёРµ", "secondary", `{"cmd":"admin_edit_setting","key":"welcome_message"}`),
				btn("РўРµРєСЃС‚ СЃРѕРіР»Р°СЃРёСЏ", "secondary", `{"cmd":"admin_edit_setting","key":"consent_text"}`),
			},
			{
				btn("РЈРїСЂР°РІР»РµРЅРёРµ FAQ", "secondary", `{"cmd":"admin_manage_faq"}`),
				btn("Р’РѕРїСЂРѕСЃС‹ Р°РЅРєРµС‚С‹", "secondary", `{"cmd":"admin_manage_questions"}`),
			},
			{
				btn("РџСЂРѕРјРїС‚ РРіСЂР°", "secondary", `{"cmd":"admin_edit_setting","key":"system_prompt_game"}`),
				btn("РџСЂРѕРјРїС‚ РљР°СЂС‚Р°", "secondary", `{"cmd":"admin_edit_setting","key":"system_prompt_map"}`),
			},
			{
				btn("Р›РёРјРёС‚ РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ", "secondary", `{"cmd":"admin_edit_setting","key":"default_request_limit"}`),
				btn("РљСѓР»РґР°СѓРЅ РїРѕ СѓРјРѕР»С‡Р°РЅРёСЋ", "secondary", `{"cmd":"admin_edit_setting","key":"default_cooldown_secs"}`),
			},
			{
				btn("Р’ Р°РґРјРёРЅРєСѓ", "primary", `{"cmd":"admin_panel"}`),
			},
		},
	}
}

func AdminChatMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("Панель админа", "primary", `{"cmd":"admin_panel"}`),
				btn("Поддержка", "secondary", `{"cmd":"support"}`),
			},
			{
				btn("Очистить Зеркало", "negative", `{"cmd":"admin_clear_mirror"}`),
				btn("Очистить Карту", "negative", `{"cmd":"admin_clear_map"}`),
			},
		},
	}
}

func ModeratorMenu() *Keyboard {
	return &Keyboard{
		OneTime: false,
		Buttons: [][]Button{
			{
				btn("РџРѕРґРґРµСЂР¶РєР°", "primary", `{"cmd":"mod_support"}`),
				btn("РЎРѕР·РґР°С‚СЊ СЃСЃС‹Р»РєСѓ", "secondary", `{"cmd":"mod_invite"}`),
			},
			{
				btn("РџСЂРѕРІРµСЂРёС‚СЊ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ", "secondary", `{"cmd":"mod_check_user"}`),
			},
		},
	}
}

func YesNo(yesPayload, noPayload string) *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{
				btn("Р”Р°", "positive", yesPayload),
				btn("РќРµС‚", "negative", noPayload),
			},
		},
	}
}

func BackOnly() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{btn("РќР°Р·Р°Рґ", "secondary", `{"cmd":"back"}`)},
		},
	}
}

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
				btn("РћРґРѕР±СЂРёС‚СЊ "+displayName, "positive", approvePayload),
				btn("РћС‚РєР»РѕРЅРёС‚СЊ "+displayName, "negative", rejectPayload),
			},
		},
	}
}

func RequestAccess() *Keyboard {
	return &Keyboard{
		OneTime: true,
		Buttons: [][]Button{
			{btn("РџРѕРґР°С‚СЊ Р·Р°СЏРІРєСѓ", "primary", `{"cmd":"request_access"}`)},
		},
	}
}

func UserListInline(users []UserButton, offset int, total int) *Keyboard {
	kb := &Keyboard{Inline: true}
	row := []Button{}
	for i, u := range users {
		label := u.Name
		if len(label) > 20 {
			label = label[:18] + "..."
		}
		payload := fmt.Sprintf(`{"cmd":"admin_user_detail","vk_id":%d}`, u.VKID)
		row = append(row, btn(label, "secondary", payload))
		if len(row) == 2 || i == len(users)-1 {
			kb.Buttons = append(kb.Buttons, row)
			row = []Button{}
		}
	}

	nav := []Button{}
	if offset > 0 {
		nav = append(nav, btn("РќР°Р·Р°Рґ", "secondary", fmt.Sprintf(`{"cmd":"admin_users_page","offset":%d}`, offset-8)))
	}
	if offset+len(users) < total {
		nav = append(nav, btn("Р”Р°Р»РµРµ", "secondary", fmt.Sprintf(`{"cmd":"admin_users_page","offset":%d}`, offset+8)))
	}
	if len(nav) > 0 {
		kb.Buttons = append(kb.Buttons, nav)
	}

	return kb
}

type UserButton struct {
	VKID int64
	Name string
}

func UserActionsInline(vkID int64) *Keyboard {
	return &Keyboard{
		Inline: true,
		Buttons: [][]Button{
			{
				btn("Р‘Р°РЅ", "negative", fmt.Sprintf(`{"cmd":"admin_ban","vk_id":%d}`, vkID)),
				btn("РћС…Р»Р°РґРёС‚СЊ 1С‡", "secondary", fmt.Sprintf(`{"cmd":"admin_cool","vk_id":%d,"mins":60}`, vkID)),
			},
			{
				btn("Р Р°Р·Р±Р°РЅРёС‚СЊ", "positive", fmt.Sprintf(`{"cmd":"admin_unban","vk_id":%d}`, vkID)),
				btn("РњРѕРґРµСЂР°С‚РѕСЂ", "primary", fmt.Sprintf(`{"cmd":"admin_set_mod","vk_id":%d}`, vkID)),
			},
			{
				btn("РЎРЅСЏС‚СЊ СЂРѕР»СЊ", "secondary", fmt.Sprintf(`{"cmd":"admin_set_user","vk_id":%d}`, vkID)),
				btn("Р›РёРјРёС‚ Р·Р°РїСЂРѕСЃРѕРІ", "secondary", fmt.Sprintf(`{"cmd":"admin_set_limit","vk_id":%d}`, vkID)),
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

func MakeBtn(label, color, payload string) Button {
	return btn(label, color, payload)
}
