// Package handlers содержит все обработчики событий бота
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/api/params"
	"github.com/vitaly06/ai-vk-bot/internal/bot/keyboards"
	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository"
	aiSvc "github.com/vitaly06/ai-vk-bot/internal/services/ai"
	inviteSvc "github.com/vitaly06/ai-vk-bot/internal/services/invite"
	monSvc "github.com/vitaly06/ai-vk-bot/internal/services/monitoring"
	paymentSvc "github.com/vitaly06/ai-vk-bot/internal/services/payment"
	userSvc "github.com/vitaly06/ai-vk-bot/internal/services/user"
)

// Handlers агрегирует все обработчики
type Handlers struct {
	Admin     *AdminHandler
	Moderator *ModeratorHandler
	User      *UserHandler
	Guest     *GuestHandler
	vk        *api.VK
	userSvc   *userSvc.Service
	mon       *monSvc.Service
	adminIDs  []int64
}

func New(
	vk *api.VK,
	uSvc *userSvc.Service,
	iSvc *inviteSvc.Service,
	aiSvc *aiSvc.Service,
	pSvc *paymentSvc.Service,
	dialogRepo repository.DialogRepository,
	settingsRepo repository.SettingsRepository,
	mon *monSvc.Service,
	adminIDs []int64,
) *Handlers {
	base := &baseHandler{vk: vk}

	h := &Handlers{
		vk:       vk,
		userSvc:  uSvc,
		mon:      mon,
		adminIDs: adminIDs,
	}

	h.Admin = &AdminHandler{base: base, userSvc: uSvc, inviteSvc: iSvc, aiSvc: aiSvc, dialogRepo: dialogRepo, settingsRepo: settingsRepo, mon: mon, handlers: h}
	h.Moderator = &ModeratorHandler{base: base, userSvc: uSvc, inviteSvc: iSvc, handlers: h}
	h.User = &UserHandler{base: base, userSvc: uSvc, aiSvc: aiSvc, paymentSvc: pSvc, dialogRepo: dialogRepo, settingsRepo: settingsRepo, mon: mon}
	h.Guest = &GuestHandler{base: base, userSvc: uSvc, inviteSvc: iSvc, handlers: h}

	return h
}

// NotifyAdmins рассылает сообщение с клавиатурой всем администраторам и модераторам
func (h *Handlers) NotifyAdmins(ctx context.Context, text string, kb *keyboards.Keyboard) {
	for _, id := range h.adminIDs {
		send(h.vk, id, text, kb)
	}
}

// SendRestricted уведомляет пользователя об ограничении
func (h *Handlers) SendRestricted(ctx context.Context, vkID int64, u *models.User) {
	msg := "⛔ Ваш аккаунт ограничен."
	if u.BannedUntil != nil {
		msg = fmt.Sprintf("⛔ Вы в режиме охлаждения до %s.", u.BannedUntil.Format("02.01.2006 15:04"))
	}
	send(h.vk, vkID, msg, nil)
}

// baseHandler — общие методы для всех обработчиков
type baseHandler struct {
	vk *api.VK
}

func (b *baseHandler) send(ctx context.Context, vkID int64, text string, kb *keyboards.Keyboard) {
	send(b.vk, vkID, text, kb)
}

// send — вспомогательная функция отправки сообщения
func send(vk *api.VK, vkID int64, text string, kb *keyboards.Keyboard) {
	const maxVKMessageLen = 3500 // reserve headroom below VK hard limit
	parts := splitByRunes(text, maxVKMessageLen)
	if len(parts) == 0 {
		parts = []string{""}
	}

	for i, part := range parts {
		b := params.NewMessagesSendBuilder()
		b.UserID(int(vkID))
		b.Message(part)
		b.RandomID(randomID())
		// Attach keyboard only to the last chunk so controls stay visible.
		if kb != nil && i == len(parts)-1 {
			b.Keyboard(kb.Serialize())
		}
		_, err := vk.MessagesSend(b.Params)
		if err != nil {
			slog.Error("send message", "vk_id", vkID, "err", err)
			return
		}
	}
}

func splitByRunes(text string, maxRunes int) []string {
	if maxRunes <= 0 || text == "" {
		return []string{text}
	}
	if utf8.RuneCountInString(text) <= maxRunes {
		return []string{text}
	}

	runes := []rune(text)
	parts := make([]string, 0, (len(runes)/maxRunes)+1)
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

// randomID генерирует уникальный random_id для VK API
func randomID() int {
	var b [4]byte
	rand.Read(b[:])
	return int(binary.LittleEndian.Uint32(b[:]))
}
