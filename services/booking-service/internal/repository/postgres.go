package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Save(ctx context.Context, b domain.Booking) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bookings (id, rider_id, pickup, dropoff, status, cancel_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, b.ID, b.RiderID, b.Pickup, b.Dropoff, b.Status, b.CancelReason, b.CreatedAt, b.UpdatedAt)
	return err
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (domain.Booking, error) {
	var b domain.Booking
	err := r.pool.QueryRow(ctx, `
		SELECT id, rider_id, pickup, dropoff, status, cancel_reason, created_at, updated_at
		FROM bookings WHERE id = $1
	`, id).Scan(&b.ID, &b.RiderID, &b.Pickup, &b.Dropoff, &b.Status, &b.CancelReason, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, fmt.Errorf("booking %s not found", id)
	}
	return b, err
}

func (r *PostgresRepository) Update(ctx context.Context, b domain.Booking) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE bookings
		SET status = $2, cancel_reason = $3, updated_at = $4
		WHERE id = $1
	`, b.ID, b.Status, b.CancelReason, b.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("booking %s not found", b.ID)
	}
	return nil
}
