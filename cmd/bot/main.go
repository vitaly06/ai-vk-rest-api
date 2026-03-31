package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/vitaly06/ai-vk-bot/internal/bot"
	"github.com/vitaly06/ai-vk-bot/internal/bot/handlers"
	"github.com/vitaly06/ai-vk-bot/internal/config"
	"github.com/vitaly06/ai-vk-bot/internal/repository/postgres"
	"github.com/vitaly06/ai-vk-bot/internal/repository/redisstore"
	aiSvc "github.com/vitaly06/ai-vk-bot/internal/services/ai"
	inviteSvc "github.com/vitaly06/ai-vk-bot/internal/services/invite"
	monSvc "github.com/vitaly06/ai-vk-bot/internal/services/monitoring"
	paymentSvc "github.com/vitaly06/ai-vk-bot/internal/services/payment"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"

	"github.com/SevereCloud/vksdk/v3/api"
)

func main() {
	// Инициализация логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Конфигурация
	cfg := config.Load()

	// PostgreSQL
	db, err := postgres.Connect(cfg.DB.DSN())
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}

	// Запуск миграций
	if err := runMigrations(db.DB); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}

	// Redis
	redis := redisstore.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redis.Ping(context.Background()); err != nil {
		slog.Error("connect redis", "err", err)
		os.Exit(1)
	}

	// Репозитории
	userRepo := postgres.NewUserRepo(db)
	dialogRepo := postgres.NewDialogRepo(db)
	inviteRepo := postgres.NewInviteRepo(db)
	paymentRepo := postgres.NewPaymentRepo(db)
	settingsRepo := postgres.NewSettingsRepo(db)

	// VK API клиент
	vkAPI := api.NewVK(cfg.VK.Token)

	// Сервисы
	uSvc := userSvc.New(userRepo, redis)
	iSvc := inviteSvc.New(inviteRepo, groupSlug(cfg), cfg.Bot.InviteLinkTTLHrs)
	ai := aiSvc.New(cfg.AI)
	pSvc := paymentSvc.New(cfg.Payment, paymentRepo, userRepo)
	mon := monSvc.New(redis)

	// Обработчики
	adminIDs := make([]int64, len(cfg.VK.AdminIDs))
	for i, id := range cfg.VK.AdminIDs {
		adminIDs[i] = int64(id)
	}
	h := handlers.New(vkAPI, uSvc, iSvc, ai, pSvc, dialogRepo, settingsRepo, mon, adminIDs)

	// Бот
	b, err := bot.New(cfg, h, uSvc, mon)
	if err != nil {
		slog.Error("create bot", "err", err)
		os.Exit(1)
	}

	slog.Info("starting bot", "group_id", cfg.VK.GroupID, "mode", cfg.Bot.Mode)
	if err := b.Run(); err != nil {
		slog.Error("bot stopped", "err", err)
		os.Exit(1)
	}
}

// groupSlug возвращает slug сообщества (числовой ID как строка)
// В продакшне замените на красивый alias сообщества
func groupSlug(cfg *config.Config) string {
	return fmt.Sprintf("club%d", cfg.VK.GroupID)
}

// runMigrations применяет SQL миграции из файла
func runMigrations(db *sql.DB) error {
	data, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = db.Exec(string(data))
	return err
}
