package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/notification-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/notification-service/internal/messaging"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/notification-service/internal/notifier"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/idempotency"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(mustEnv("KAFKA_BROKERS"), ",")
	redisAddr := mustEnv("REDIS_ADDR")
	idemPrefix := envOr("IDEMPOTENCY_PREFIX", "notification-service")

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	idem := idempotency.NewRedisChecker(redisClient, idemPrefix)

	svc := domain.NewService(notifier.NewLog())

	log.Println("notification-service consuming events")
	messaging.Run(ctx, brokers, svc, idem)

	<-ctx.Done()
	log.Println("notification-service shutting down")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
