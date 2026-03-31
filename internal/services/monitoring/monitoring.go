package monitoring

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/vitaly06/ai-vk-bot/internal/models"
	"github.com/vitaly06/ai-vk-bot/internal/repository/redisstore"
)

// Service — сбор метрик бота
type Service struct {
	redis     *redisstore.Store
	startTime time.Time

	errorsTotal  atomic.Int64
	aiCallsToday atomic.Int64
}

func New(redis *redisstore.Store) *Service {
	return &Service{
		redis:     redis,
		startTime: time.Now(),
	}
}

func (s *Service) RecordError() {
	s.errorsTotal.Add(1)
	s.redis.IncrCounter(context.Background(), "errors:today")
}

func (s *Service) RecordAICall() {
	s.aiCallsToday.Add(1)
	s.redis.IncrCounter(context.Background(), "ai_calls:today")
}

func (s *Service) GetMetrics(ctx context.Context) models.Metrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	errToday, _ := s.redis.GetCounter(ctx, "errors:today")
	aiToday, _ := s.redis.GetCounter(ctx, "ai_calls:today")

	return models.Metrics{
		ErrorsToday:   int(errToday),
		AICallsToday:  int(aiToday),
		MemoryUsageMB: float64(mem.Alloc) / 1024 / 1024,
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
	}
}
