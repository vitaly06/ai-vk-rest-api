package models

import (
	"fmt"
	"strings"
	"time"
)

// Role определяет роль пользователя в боте
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleModerator Role = "moderator"
	RoleUser      Role = "user"
	RoleGuest     Role = "guest" // вошёл без ссылки
)

// UserStatus — статус аккаунта
type UserStatus string

const (
	StatusActive     UserStatus = "active"
	StatusBanned     UserStatus = "banned"
	StatusRestricted UserStatus = "restricted" // охлаждение
	StatusPending    UserStatus = "pending"    // ожидает одобрения
)

// BotState — текущий шаг диалога (FSM)
type BotState string

const (
	StateNone             BotState = ""
	StateWelcome          BotState = "welcome"
	StateConsent          BotState = "consent"       // согласие на обработку данных
	StateQuestionnaire    BotState = "questionnaire" // анкета
	StateMainChat         BotState = "main_chat"     // основной диалог
	StateSupport          BotState = "support"       // чат поддержки
	StateAwaitPayment     BotState = "await_payment"
	StateAwaitRequestText BotState = "await_request_text" // пользователь вводит текст заявки
	StateAwaitApproval    BotState = "await_approval"     // заявка отправлена, ожидает одобрения
)

// User — пользователь бота
type User struct {
	ID             int64      `db:"id"`
	VKID           int64      `db:"vk_id"`
	FirstName      string     `db:"first_name"`
	LastName       string     `db:"last_name"`
	Username       string     `db:"username"`
	Role           Role       `db:"role"`
	Status         UserStatus `db:"status"`
	State          BotState   `db:"state"`
	InviteID       *int64     `db:"invite_id"`       // по какой ссылке пришёл
	RequestCount   int        `db:"request_count"`   // кол-во запросов к AI
	RequestLimit   int        `db:"request_limit"`   // лимит запросов (0 = безлимит)
	BannedUntil    *time.Time `db:"banned_until"`    // до когда бан/охлаждение
	ConsentGiven   bool       `db:"consent_given"`   // согласие на перс. данные
	MailingConsent bool       `db:"mailing_consent"` // согласие на рассылку
	Balance        float64    `db:"balance"`         // внутренний кошелёк
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// QuestionnaireAnswer — ответы анкеты
type QuestionnaireAnswer struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Question  string    `db:"question"`
	Answer    string    `db:"answer"`
	CreatedAt time.Time `db:"created_at"`
}

// AccessRequest — запрос на вступление без ссылки
type AccessRequest struct {
	ID        int64     `db:"id"`
	VKID      int64     `db:"vk_id"`
	Message   string    `db:"message"`
	Status    string    `db:"status"` // pending | approved | rejected
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (u *User) DisplayName() string {
	if u == nil {
		return ""
	}

	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("id%d", u.VKID)
}
