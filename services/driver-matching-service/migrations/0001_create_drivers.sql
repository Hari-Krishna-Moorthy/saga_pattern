CREATE TABLE IF NOT EXISTS drivers (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    status               TEXT NOT NULL,
    assigned_booking_id  TEXT NOT NULL DEFAULT ''
);

-- Seed a small pool of drivers so the saga has someone to match against;
-- there's no driver-onboarding API in this demo.
INSERT INTO drivers (id, name, status)
VALUES
    ('driver-1', 'Alex Rivera', 'AVAILABLE'),
    ('driver-2', 'Sam Okafor', 'AVAILABLE'),
    ('driver-3', 'Priya Nair', 'AVAILABLE')
ON CONFLICT (id) DO NOTHING;
