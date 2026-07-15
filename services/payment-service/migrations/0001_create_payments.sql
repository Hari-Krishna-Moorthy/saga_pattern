CREATE TABLE IF NOT EXISTS payments (
    id          UUID PRIMARY KEY,
    booking_id  UUID NOT NULL,
    driver_id   TEXT NOT NULL,
    status      TEXT NOT NULL
);
