CREATE TABLE IF NOT EXISTS bookings (
    id             UUID PRIMARY KEY,
    rider_id       TEXT NOT NULL,
    pickup         TEXT NOT NULL,
    dropoff        TEXT NOT NULL,
    status         TEXT NOT NULL,
    cancel_reason  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);
