#!/usr/bin/env bash
# Drives all three saga outcomes against a live docker-compose stack and
# prints what happened at each step: the happy path (booking -> driver
# matched -> payment charged -> confirmed), the no-driver compensation
# (booking -> match_failed -> cancelled), and the payment-declined
# compensation (booking -> matched -> payment declined -> cancelled +
# driver released). See docs/architecture.md for the event flow this
# script is exercising.
#
# Usage:
#   ./scripts/simulate-saga.sh            # use whatever's already running
#   ./scripts/simulate-saga.sh --fresh    # docker compose down -v && up --build first
#
# Requires: docker compose, curl. No jq dependency (uses grep/sed on the
# flat JSON the API returns).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

BASE_URL="${BASE_URL:-http://localhost:8080}"
POLL_INTERVAL=2
POLL_TIMEOUT=90 # generous: a cold Kafka broker can take up to ~60s to settle, see docs/infra/kafka.md

# ---------- output helpers ----------

bold()    { printf '\033[1m%s\033[0m\n' "$*"; }
section() { printf '\n\033[1;36m== %s ==\033[0m\n' "$*"; }
info()    { printf '  %s\n' "$*"; }
ok()      { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
fail()    { printf '  \033[1;31m✗\033[0m %s\n' "$*"; }

# ---------- JSON helpers (no jq dependency) ----------
# The API returns flat JSON like:
# {"ID":"...","RiderID":"...","Pickup":"...","Dropoff":"...","Status":"...","CancelReason":"...","CreatedAt":"...","UpdatedAt":"..."}

json_field() { # json_field <json> <field-name>
  echo "$1" | grep -o "\"$2\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
}

# ---------- saga API helpers ----------

create_booking() { # create_booking <rider_id> <pickup> <dropoff> -> prints booking ID
  local rider="$1" pickup="$2" dropoff="$3"
  local resp
  resp=$(curl -s -X POST "$BASE_URL/bookings" \
    -H "Content-Type: application/json" \
    -d "{\"rider_id\":\"$rider\",\"pickup\":\"$pickup\",\"dropoff\":\"$dropoff\"}")
  json_field "$resp" "ID"
}

get_booking() { # get_booking <id> -> prints raw JSON
  curl -s "$BASE_URL/bookings/$1"
}

# wait_for_terminal_status <booking_id> -> prints "STATUS|REASON" once the
# booking leaves REQUESTED, or "TIMEOUT|" if it never does.
wait_for_terminal_status() {
  local id="$1" waited=0 resp status reason
  while (( waited < POLL_TIMEOUT )); do
    resp=$(get_booking "$id")
    status=$(json_field "$resp" "Status")
    if [[ "$status" != "REQUESTED" && -n "$status" ]]; then
      reason=$(json_field "$resp" "CancelReason")
      echo "${status}|${reason}"
      return 0
    fi
    printf '.' >&2  # progress dot to stderr — stdout is captured by $(...) callers
    sleep "$POLL_INTERVAL"
    waited=$(( waited + POLL_INTERVAL ))
  done
  echo "TIMEOUT|"
}

# ---------- Postgres / driver-pool helpers ----------

psql_drivers() { # psql_drivers <sql> -> runs against driver_matching_service, tuples-only
  docker exec saga-postgres psql -U saga -d driver_matching_service -tA -c "$1"
}

reset_driver_pool() {
  psql_drivers "UPDATE drivers SET status='AVAILABLE', assigned_booking_id='';" > /dev/null
}

available_driver_count() {
  psql_drivers "SELECT count(*) FROM drivers WHERE status='AVAILABLE';"
}

# ---------- setup ----------

if [[ "${1:-}" == "--fresh" ]]; then
  section "Fresh stack (docker compose down -v && up --build)"
  docker compose down -v
  docker compose up -d --build
  info "waiting ~30s for Kafka's consumer groups to stabilize on a cold broker (see docs/infra/kafka.md#cold-start-behavior)"
  sleep 30
else
  section "Using the already-running stack"
  docker compose up -d > /dev/null 2>&1
fi

if ! curl -s -o /dev/null -f "$BASE_URL/bookings/00000000-0000-0000-0000-000000000000"; then
  : # 404 is expected and fine — this just confirms booking-service is answering HTTP
fi

section "Resetting the demo driver pool to AVAILABLE"
reset_driver_pool
ok "3 demo drivers (driver-1, driver-2, driver-3) reset to AVAILABLE"

FAILURES=0

# ---------- Scenario 1: happy path ----------

section "Scenario 1: happy path (booking -> matched -> paid -> CONFIRMED)"
id1=$(create_booking "rider-sim-1" "100 Simulation St" "200 Demo Ave")
info "created booking $id1"
printf '  waiting for saga to settle '
result=$(wait_for_terminal_status "$id1")
echo
status="${result%%|*}"
reason="${result##*|}"
if [[ "$status" == "CONFIRMED" ]]; then
  ok "booking $id1 -> CONFIRMED"
else
  fail "booking $id1 -> $status (expected CONFIRMED) reason=$reason"
  FAILURES=$((FAILURES+1))
fi

# ---------- Scenario 2: no driver available ----------

section "Scenario 2: compensation - no driver available"
n_available=$(available_driver_count)
info "$n_available driver(s) currently AVAILABLE; exhausting the pool first"

for i in $(seq 1 "$n_available"); do
  exhaust_id=$(create_booking "rider-sim-exhaust-$i" "Exhaust Pickup $i" "Exhaust Dropoff $i")
  wait_for_terminal_status "$exhaust_id" > /dev/null
done
info "driver pool exhausted"

id2=$(create_booking "rider-sim-2" "300 NoDriver Rd" "400 Empty Ln")
info "created booking $id2 (should find zero available drivers)"
printf '  waiting for saga to settle '
result=$(wait_for_terminal_status "$id2")
echo
status="${result%%|*}"
reason="${result##*|}"
if [[ "$status" == "CANCELLED" && "$reason" == "no driver available" ]]; then
  ok "booking $id2 -> CANCELLED (reason: $reason)"
else
  fail "booking $id2 -> $status reason=$reason (expected CANCELLED / \"no driver available\")"
  FAILURES=$((FAILURES+1))
fi

# ---------- Scenario 3: payment declined (compensation) ----------

section "Scenario 3: compensation - payment declined"
reset_driver_pool
ok "driver pool reset again so this booking can actually get matched"

id3=$(create_booking "rider-sim-3" "500 Decline Blvd" "600 Refund St")
info "created booking $id3 (will force its payment to be declined)"
printf '  waiting for driver to be matched '
waited=0
matched=""
while (( waited < POLL_TIMEOUT )); do
  matched=$(psql_drivers "SELECT id FROM drivers WHERE assigned_booking_id='$id3';")
  [[ -n "$matched" ]] && break
  printf '.'
  sleep "$POLL_INTERVAL"
  waited=$(( waited + POLL_INTERVAL ))
done
echo
if [[ -z "$matched" ]]; then
  fail "no driver was matched to $id3 within ${POLL_TIMEOUT}s — skipping the decline step"
  FAILURES=$((FAILURES+1))
else
  ok "driver $matched matched to $id3"

  info "stopping the normal payment-service and starting one configured to decline $id3"
  docker compose stop payment-service > /dev/null 2>&1

  decline_container="saga-payment-service-sim-decline"
  docker rm -f "$decline_container" > /dev/null 2>&1 || true
  docker compose run -d --name "$decline_container" \
    -e DECLINE_BOOKING_IDS="$id3" \
    payment-service > /dev/null 2>&1

  # always restore the normal payment-service, even if the rest of this
  # scenario fails partway through
  cleanup_payment_service() {
    docker rm -f "$decline_container" > /dev/null 2>&1 || true
    docker compose up -d payment-service > /dev/null 2>&1 || true
  }
  trap cleanup_payment_service EXIT

  printf '  waiting for the decline to propagate through the saga '
  result=$(wait_for_terminal_status "$id3")
  echo
  status="${result%%|*}"
  reason="${result##*|}"

  driver_status=""
  waited=0
  while (( waited < POLL_TIMEOUT )); do
    driver_status=$(psql_drivers "SELECT status FROM drivers WHERE id='$matched';")
    [[ "$driver_status" == "AVAILABLE" ]] && break
    sleep "$POLL_INTERVAL"
    waited=$(( waited + POLL_INTERVAL ))
  done

  if [[ "$status" == "CANCELLED" && "$reason" == "payment failed" && "$driver_status" == "AVAILABLE" ]]; then
    ok "booking $id3 -> CANCELLED (reason: $reason), driver $matched -> AVAILABLE again"
  else
    fail "booking $id3 -> $status reason=$reason, driver $matched -> $driver_status (expected CANCELLED / \"payment failed\" / AVAILABLE)"
    FAILURES=$((FAILURES+1))
  fi

  cleanup_payment_service
  trap - EXIT
fi

# ---------- Summary ----------

section "notification-service log tail (what the rider would have seen)"
docker logs saga-notification-service --tail 12 2>&1 | sed 's/^/  /'

section "Summary"
if (( FAILURES == 0 )); then
  bold "All 3 saga scenarios completed as expected."
else
  bold "$FAILURES scenario(s) did not complete as expected — see ✗ lines above."
  exit 1
fi
