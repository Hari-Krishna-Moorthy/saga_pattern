package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/driver-matching-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindAvailableDriver(ctx context.Context) (domain.Driver, error) {
	var d domain.Driver
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, status, assigned_booking_id
		FROM drivers WHERE status = $1
		ORDER BY id LIMIT 1
	`, domain.DriverAvailable).Scan(&d.ID, &d.Name, &d.Status, &d.AssignedBookingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Driver{}, domain.ErrNoDriverAvailable
	}
	return d, err
}

func (r *PostgresRepository) AssignDriver(ctx context.Context, driverID, bookingID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE drivers SET status = $2, assigned_booking_id = $3
		WHERE id = $1 AND status = $4
	`, driverID, domain.DriverMatched, bookingID, domain.DriverAvailable)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("driver %s was not available to assign", driverID)
	}
	return nil
}

func (r *PostgresRepository) FindByBookingID(ctx context.Context, bookingID string) (domain.Driver, error) {
	var d domain.Driver
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, status, assigned_booking_id
		FROM drivers WHERE assigned_booking_id = $1
	`, bookingID).Scan(&d.ID, &d.Name, &d.Status, &d.AssignedBookingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Driver{}, fmt.Errorf("no driver assigned to booking %s", bookingID)
	}
	return d, err
}

func (r *PostgresRepository) ReleaseDriver(ctx context.Context, driverID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE drivers SET status = $2, assigned_booking_id = ''
		WHERE id = $1
	`, driverID, domain.DriverAvailable)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("driver %s not found", driverID)
	}
	return nil
}
