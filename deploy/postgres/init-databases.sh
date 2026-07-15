#!/usr/bin/env bash
# Creates one database per service (database-per-service isolation) on
# first container start. Runs automatically because it's mounted into
# /docker-entrypoint-initdb.d.
set -euo pipefail

for db in booking_service driver_matching_service payment_service; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    SELECT 'CREATE DATABASE $db'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
EOSQL
done
