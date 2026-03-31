package models

import "time"

// BotSetting — глобальные настройки бота
type BotSetting struct {
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Известные ключи настроек
const (
	SettingWelcomeMessage      = "welcome_message"
	SettingConsentText         = "consent_text"
	SettingDefaultCooldownSecs = "default_cooldown_secs"
	SettingDefaultRequestLimit = "default_request_limit"
	SettingRegistrationOpen    = "registration_open"
	SettingMainMenuText        = "main_menu_text"
	SettingFAQText             = "faq_text"
	SettingAboutText           = "about_text"
)

// Metrics — текущие метрики для мониторинга
type Metrics struct {
	ActiveUsers   int     `json:"active_users"`
	TotalUsers    int     `json:"total_users"`
	MessagesToday int     `json:"messages_today"`
	ErrorsToday   int     `json:"errors_today"`
	AICallsToday  int     `json:"ai_calls_today"`
	MemoryUsageMB float64 `json:"memory_usage_mb"`
	UptimeSeconds int64   `json:"uptime_seconds"`
}

// AuditLog — журнал действий администраторов/модераторов
type AuditLog struct {
	ID        int64     `db:"id"`
	ActorID   int64     `db:"actor_id"`  // VKID того, кто совершил действие
	TargetID  *int64    `db:"target_id"` // VKID объекта действия
	Action    string    `db:"action"`    // ban | unban | restrict | delete_message | etc.
	Details   string    `db:"details"`   // JSON или текст
	CreatedAt time.Time `db:"created_at"`
}
