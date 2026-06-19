package models

import (
	"fmt"
	"strings"
	"time"
)

// Role defines the bot role.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleModerator Role = "moderator"
	RoleUser      Role = "user"
	RoleGuest     Role = "guest"
)

// UserStatus defines the account status.
type UserStatus string

const (
	StatusActive     UserStatus = "active"
	StatusBanned     UserStatus = "banned"
	StatusRestricted UserStatus = "restricted"
	StatusPending    UserStatus = "pending"
)

// BotState defines the current FSM state.
type BotState string

const (
	StateNone             BotState = ""
	StateWelcome          BotState = "welcome"
	StateConsent          BotState = "consent"
	StateQuestionnaire    BotState = "questionnaire"
	StateMainChat         BotState = "main_chat"
	StateMapChat          BotState = "map_chat"
	StateSupport          BotState = "support"
	StateAwaitPayment     BotState = "await_payment"
	StateAwaitRequestText BotState = "await_request_text"
	StateAwaitApproval    BotState = "await_approval"
)

// User stores a bot user profile.
type User struct {
	ID             int64      `db:"id"`
	VKID           int64      `db:"vk_id"`
	FirstName      string     `db:"first_name"`
	LastName       string     `db:"last_name"`
	Username       string     `db:"username"`
	Role           Role       `db:"role"`
	Status         UserStatus `db:"status"`
	State          BotState   `db:"state"`
	InviteID       *int64     `db:"invite_id"`
	RequestCount   int        `db:"request_count"`
	RequestLimit   int        `db:"request_limit"`
	BannedUntil    *time.Time `db:"banned_until"`
	ConsentGiven   bool       `db:"consent_given"`
	MailingConsent bool       `db:"mailing_consent"`
	Balance        float64    `db:"balance"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// QuestionnaireAnswer stores a questionnaire answer.
type QuestionnaireAnswer struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Question  string    `db:"question"`
	Answer    string    `db:"answer"`
	CreatedAt time.Time `db:"created_at"`
}

// AccessRequest stores an access request for invite-less users.
type AccessRequest struct {
	ID        int64     `db:"id"`
	VKID      int64     `db:"vk_id"`
	Message   string    `db:"message"`
	Status    string    `db:"status"`
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
