package repository

import (
	"context"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/payment-service/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Save(ctx context.Context, p domain.Payment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, booking_id, driver_id, status)
		VALUES ($1, $2, $3, $4)
	`, p.ID, p.BookingID, p.DriverID, p.Status)
	return err
}
