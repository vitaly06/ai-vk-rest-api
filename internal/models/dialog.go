package models

import "time"

// Dialog — ветвь диалога конкретного пользователя с AI
type Dialog struct {
	ID        int64      `db:"id"`
	UserID    int64      `db:"user_id"`
	Type      DialogType `db:"type"` // main | support
	IsActive  bool       `db:"is_active"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

type DialogType string

const (
	DialogMain    DialogType = "main"
	DialogSupport DialogType = "support"
)

// MessageRole — роль отправителя сообщения
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleModerator MessageRole = "moderator"
	MessageRoleSystem    MessageRole = "system"
)

// MessageType — тип вложения
type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeAudio MessageType = "audio"
	MessageTypeVideo MessageType = "video"
)

// Message — сообщение в диалоге
type Message struct {
	ID          int64       `db:"id"`
	DialogID    int64       `db:"dialog_id"`
	UserID      int64       `db:"user_id"`
	Role        MessageRole `db:"role"`
	Type        MessageType `db:"type"`
	Content     string      `db:"content"`       // текст или URL вложения
	VKMessageID int         `db:"vk_message_id"` // ID сообщения в VK (для закрепления/удаления)
	IsPinned    bool        `db:"is_pinned"`
	IsDeleted   bool        `db:"is_deleted"`
	CreatedAt   time.Time   `db:"created_at"`
}

// AIMessage — формат для передачи истории в AI
type AIMessage struct {
	Role    string `json:"role"` // user | assistant | system
	Content string `json:"content"`
}
