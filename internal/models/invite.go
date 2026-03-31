package models

import "time"

// Invite — уникальная ссылка-приглашение
type Invite struct {
	ID          int64      `db:"id"`
	Token       string     `db:"token"`         // UUID токен ссылки
	CreatedByID int64      `db:"created_by_id"` // VKID админа/модератора
	UsedByID    *int64     `db:"used_by_id"`    // VKID пользователя, использовавшего ссылку
	MaxUses     int        `db:"max_uses"`      // 0 = одноразовая
	UsesCount   int        `db:"uses_count"`
	ExpiresAt   *time.Time `db:"expires_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

// IsValid проверяет, действительна ли ссылка
func (i *Invite) IsValid() bool {
	if i.ExpiresAt != nil && time.Now().After(*i.ExpiresAt) {
		return false
	}
	if i.MaxUses > 0 && i.UsesCount >= i.MaxUses {
		return false
	}
	return true
}

// InviteLink — сформированная ссылка для отправки пользователю
// VK deep link: https://vk.me/your_community?ref=<token>
type InviteLink struct {
	Invite *Invite
	URL    string
}
