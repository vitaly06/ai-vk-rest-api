package redisstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vitaly06/ai-vk-bot/internal/models"
)

type Store struct {
	client *redis.Client
}

func New(addr, password string, db int) *Store {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Store{client: client}
}

// Ping проверяет соединение
func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// --- Корзина ---

func cartKey(vkID int64) string {
	return fmt.Sprintf("cart:%d", vkID)
}

func (s *Store) GetCart(ctx context.Context, vkID int64) ([]models.CartItem, error) {
	data, err := s.client.Get(ctx, cartKey(vkID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []models.CartItem
	err = json.Unmarshal(data, &items)
	return items, err
}

func (s *Store) SetCart(ctx context.Context, vkID int64, items []models.CartItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, cartKey(vkID), data, 24*time.Hour).Err()
}

func (s *Store) ClearCart(ctx context.Context, vkID int64) error {
	return s.client.Del(ctx, cartKey(vkID)).Err()
}

// --- Rate limiting (количество запросов к AI) ---

func rateLimitKey(vkID int64) string {
	return fmt.Sprintf("ratelimit:%d", vkID)
}

// IncrRequestCount увеличивает счётчик запросов пользователя за сегодня.
// Возвращает текущее количество запросов.
func (s *Store) IncrRequestCount(ctx context.Context, vkID int64) (int64, error) {
	key := rateLimitKey(vkID)
	cnt, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Устанавливаем TTL до конца текущих суток
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	s.client.ExpireAt(ctx, key, midnight)
	return cnt, nil
}

// --- Сессии / временные данные ---

func sessionKey(vkID int64) string {
	return fmt.Sprintf("session:%d", vkID)
}

func (s *Store) SetSessionData(ctx context.Context, vkID int64, data map[string]string, ttl time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sessionKey(vkID), b, ttl).Err()
}

func (s *Store) GetSessionData(ctx context.Context, vkID int64) (map[string]string, error) {
	b, err := s.client.Get(ctx, sessionKey(vkID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data map[string]string
	err = json.Unmarshal(b, &data)
	return data, err
}

func (s *Store) DeleteSessionData(ctx context.Context, vkID int64) error {
	return s.client.Del(ctx, sessionKey(vkID)).Err()
}

// --- Метрики (простые счётчики) ---

func (s *Store) IncrCounter(ctx context.Context, key string) error {
	return s.client.Incr(ctx, key).Err()
}

func (s *Store) GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := s.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}
