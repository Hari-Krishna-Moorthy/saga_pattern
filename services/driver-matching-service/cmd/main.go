package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/domain"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/messaging"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/repository"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/migrations"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/idempotency"
	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/shared/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := mustEnv("POSTGRES_DSN")
	brokers := strings.Split(mustEnv("KAFKA_BROKERS"), ",")
	redisAddr := mustEnv("REDIS_ADDR")
	idemPrefix := envOr("IDEMPOTENCY_PREFIX", "driver-matching-service")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pool.Close()

	if err := runMigrations(ctx, pool); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	idem := idempotency.NewRedisChecker(redisClient, idemPrefix)

	repo := repository.NewPostgresRepository(pool)
	publisher := kafka.NewWriter(brokers)
	defer publisher.Close()

	svc := domain.NewService(repo, publisher)

	log.Println("driver-matching-service consuming events")
	messaging.Run(ctx, brokers, svc, idem)

	<-ctx.Done()
	log.Println("driver-matching-service shutting down")
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

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		sqlBytes, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}
