#!/usr/bin/env bash
# =============================================================================
# Dating Bot — Notification Service Test Suite
# =============================================================================
# Covers:
#   1. Health checks
#   2. Gateway /internal/notify (direct delivery)
#   3. match.created  → RabbitMQ → notification saved in DB
#   4. interaction.liked → RabbitMQ → notification saved in DB
#   5. chat.message_sent → RabbitMQ → notification saved in DB
#   6. REST API: GET /api/v1/notifications/{user_id}
#   7. REST API: POST /api/v1/notifications/{id}/read
#   8. Prometheus /metrics endpoint
#
# Prerequisites:
#   brew install curl jq
#   All services running (user-profile, matching, chat, gateway, notification)
#
# Usage:
#   chmod +x tests/notification_test.sh
#   ./tests/notification_test.sh
# =============================================================================

set -euo pipefail

# ── Colour helpers ─────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── Service URLs ───────────────────────────────────────────────────────────────
USER_PROFILE_URL="${USER_PROFILE_URL:-http://localhost:8080}"
MATCHING_URL="${MATCHING_URL:-http://localhost:8081}"
CHAT_URL="${CHAT_URL:-http://localhost:8084}"
NOTIFICATION_URL="${NOTIFICATION_URL:-http://localhost:8087}"
GATEWAY_INTERNAL_URL="${GATEWAY_INTERNAL_URL:-http://localhost:8086}"
RABBITMQ_API="${RABBITMQ_API:-http://localhost:15672}"
RABBITMQ_USER="${RABBITMQ_USER:-guest}"
RABBITMQ_PASS="${RABBITMQ_PASS:-guest}"
RABBITMQ_EXCHANGE="${RABBITMQ_EXCHANGE:-dating.events}"

# ── Test counters ──────────────────────────────────────────────────────────────
PASSED=0
FAILED=0
SKIPPED=0

# ── Shared state ───────────────────────────────────────────────────────────────
RUN_ID=$(date +%s)
TG_A=$((RUN_ID % 900000000 + 100000000))
TG_B=$((TG_A + 1))

USER_A_ID=""   # internal user_id for user A
USER_B_ID=""   # internal user_id for user B
MATCH_ID=""
CONV_ID=""
NOTIF_ID=""    # used in mark-as-read test

# ── Helpers ────────────────────────────────────────────────────────────────────

log_section() {
    echo ""
    echo -e "${BOLD}${BLUE}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${BLUE}  $1${NC}"
    echo -e "${BOLD}${BLUE}══════════════════════════════════════════════════════${NC}"
}

log_test() { echo -e "${CYAN}  ▶  $1${NC}"; }

pass() { echo -e "  ${GREEN}✔  PASS${NC} — $1"; PASSED=$((PASSED + 1)); }
fail() { echo -e "  ${RED}✘  FAIL${NC} — $1"; FAILED=$((FAILED + 1)); }
skip() { echo -e "  ${YELLOW}⊘  SKIP${NC} — $1"; SKIPPED=$((SKIPPED + 1)); }

assert_eq() {
    [ "$1" = "$2" ] && pass "$3 (got: $1)" || fail "$3 — expected '$2', got '$1'"
}

assert_not_empty() {
    [ -n "$1" ] && [ "$1" != "null" ] && [ "$1" != "0" ] \
        && pass "$2 (got: $1)" || fail "$2 — value is empty/null/zero"
}

assert_gt() {
    awk "BEGIN{exit !($1 > $2)}" \
        && pass "$3 ($1 > $2)" || fail "$3 — expected $1 > $2"
}

http_get()  { curl -s --max-time 10 "$1"; }
http_post() { curl -s --max-time 10 -X POST -H "Content-Type: application/json" -d "$2" "$1"; }
http_status() { curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$1"; }

wait_async() {
    echo -e "  ${YELLOW}  ⏳ Waiting ${2:-2}s for async: $1${NC}"
    sleep "${2:-2}"
}

# Publish a raw JSON event to RabbitMQ via the Management HTTP API.
rabbitmq_publish() {
    local routing_key="$1"
    local payload="$2"

    local escaped
    escaped=$(echo "$payload" | sed 's/"/\\"/g')

    curl -s -o /dev/null -w "%{http_code}" \
        -u "${RABBITMQ_USER}:${RABBITMQ_PASS}" \
        -H "Content-Type: application/json" \
        -X POST \
        "${RABBITMQ_API}/api/exchanges/%2F/${RABBITMQ_EXCHANGE}/publish" \
        -d "{
              \"properties\":{\"content_type\":\"application/json\",\"delivery_mode\":2},
              \"routing_key\":\"${routing_key}\",
              \"payload\":\"${escaped}\",
              \"payload_encoding\":\"string\"
            }"
}

# ── Dependency check ───────────────────────────────────────────────────────────

check_dependencies() {
    log_section "Dependency Check"

    log_test "curl available"
    command -v curl &>/dev/null && pass "curl found" || { fail "curl not found — install curl"; exit 1; }

    log_test "jq available"
    command -v jq &>/dev/null && pass "jq found" || { fail "jq not found — brew install jq"; exit 1; }
}

# ── Suite 1: Health Checks ─────────────────────────────────────────────────────

suite_health() {
    log_section "Suite 1: Health Checks"

    log_test "notification-service /health"
    assert_eq "$(http_status "${NOTIFICATION_URL}/health")" "200" "notification-service is healthy"

    log_test "gateway-service internal HTTP"
    # Gateway /internal/notify only responds to POST — a GET returns 405 which proves it's alive.
    local gw_status
    gw_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        -X GET "${GATEWAY_INTERNAL_URL}/internal/notify" 2>/dev/null || echo "000")
    [ "$gw_status" != "000" ] \
        && pass "gateway internal HTTP reachable (status: $gw_status)" \
        || fail "gateway internal HTTP not reachable"

    log_test "RabbitMQ Management API"
    local rmq_status
    rmq_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
        -u "${RABBITMQ_USER}:${RABBITMQ_PASS}" \
        "${RABBITMQ_API}/api/overview")
    assert_eq "$rmq_status" "200" "RabbitMQ Management API is up"
}

# ── Suite 2: Gateway Direct Delivery ──────────────────────────────────────────

suite_gateway_delivery() {
    log_section "Suite 2: Gateway Direct Delivery"

    log_test "POST /internal/notify with invalid body"
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST -H "Content-Type: application/json" \
        -d '{"telegram_id":0}' \
        "${GATEWAY_INTERNAL_URL}/internal/notify")
    assert_eq "$status" "400" "missing text returns 400"

    log_test "POST /internal/notify with fake telegram_id"
    # Should reach gateway but Telegram will reject the unknown chat.
    # We only check that the gateway itself responds (not that Telegram delivered).
    local resp
    resp=$(http_post "${GATEWAY_INTERNAL_URL}/internal/notify" \
        '{"telegram_id":999999999,"text":"test notification from test suite"}')
    local has_field
    has_field=$(echo "$resp" | jq 'has("ok") or has("error")' 2>/dev/null || echo "false")
    assert_eq "$has_field" "true" "gateway returns ok or error field"
}

# ── Suite 3: Register Users (needed for event resolution) ────────────────────

suite_register_users() {
    log_section "Suite 3: Register Test Users"

    log_test "register user A (telegram_id=${TG_A})"
    local resp_a
    resp_a=$(http_post "${USER_PROFILE_URL}/api/v1/users/register" \
        "{\"telegramId\":${TG_A},\"username\":\"notif_test_a_${RUN_ID}\",\"firstName\":\"NotifA\"}")
    USER_A_ID=$(echo "$resp_a" | jq -r '.user.id // empty')
    assert_not_empty "$USER_A_ID" "user A created with id"

    log_test "register user B (telegram_id=${TG_B})"
    local resp_b
    resp_b=$(http_post "${USER_PROFILE_URL}/api/v1/users/register" \
        "{\"telegramId\":${TG_B},\"username\":\"notif_test_b_${RUN_ID}\",\"firstName\":\"NotifB\"}")
    USER_B_ID=$(echo "$resp_b" | jq -r '.user.id // empty')
    assert_not_empty "$USER_B_ID" "user B created with id"
}

# ── Suite 4: match.created Event ──────────────────────────────────────────────

suite_match_created() {
    log_section "Suite 4: match.created → notifications for both users"

    if [ -z "$USER_A_ID" ] || [ -z "$USER_B_ID" ]; then
        skip "user IDs not available — skipping match.created suite"
        return
    fi

    MATCH_ID=$((RUN_ID % 100000 + 1))

    local payload
    payload=$(printf '{"event_name":"match.created","timestamp":"%s","data":{"match_id":%d,"user1_id":%s,"user2_id":%s}}' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$MATCH_ID" "$USER_A_ID" "$USER_B_ID")

    log_test "publish match.created to RabbitMQ"
    local rmq_status
    rmq_status=$(rabbitmq_publish "match.created" "$payload")
    assert_eq "$rmq_status" "200" "RabbitMQ accepted match.created"

    wait_async "notification-service to process match.created" 3

    log_test "user A has match_created notification"
    local resp_a
    resp_a=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID}")
    local count_a
    count_a=$(echo "$resp_a" | jq '[.notifications[]? | select(.Type=="match_created")] | length' 2>/dev/null || echo 0)
    assert_gt "$count_a" "0" "user A has at least 1 match_created notification"

    log_test "user B has match_created notification"
    local resp_b
    resp_b=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_B_ID}")
    local count_b
    count_b=$(echo "$resp_b" | jq '[.notifications[]? | select(.Type=="match_created")] | length' 2>/dev/null || echo 0)
    assert_gt "$count_b" "0" "user B has at least 1 match_created notification"

    log_test "unread_count is present in response"
    local unread
    unread=$(echo "$resp_a" | jq '.unread_count // -1')
    assert_gt "$unread" "-1" "unread_count field present"

    # Save a notification ID for the mark-as-read test.
    NOTIF_ID=$(echo "$resp_a" | jq -r '.notifications[0].ID // .notifications[0].id // empty' 2>/dev/null || echo "")
}

# ── Suite 5: interaction.liked Event ──────────────────────────────────────────

suite_interaction_liked() {
    log_section "Suite 5: interaction.liked → notification for to_user_id"

    if [ -z "$USER_A_ID" ] || [ -z "$USER_B_ID" ]; then
        skip "user IDs not available — skipping interaction.liked suite"
        return
    fi

    local payload
    payload=$(printf '{"event_name":"interaction.liked","timestamp":"%s","data":{"from_user_id":%s,"to_user_id":%s}}' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$USER_A_ID" "$USER_B_ID")

    log_test "publish interaction.liked to RabbitMQ (A liked B)"
    local rmq_status
    rmq_status=$(rabbitmq_publish "interaction.liked" "$payload")
    assert_eq "$rmq_status" "200" "RabbitMQ accepted interaction.liked"

    wait_async "notification-service to process interaction.liked" 3

    log_test "user B (to_user_id) has new_like notification"
    local resp_b
    resp_b=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_B_ID}")
    local count_b
    count_b=$(echo "$resp_b" | jq '[.notifications[]? | select(.Type=="new_like")] | length' 2>/dev/null || echo 0)
    assert_gt "$count_b" "0" "user B has at least 1 new_like notification"

    log_test "user A (from_user_id) has NO new_like notification"
    local resp_a
    resp_a=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID}")
    local count_a
    count_a=$(echo "$resp_a" | jq '[.notifications[]? | select(.Type=="new_like")] | length' 2>/dev/null || echo 0)
    assert_eq "$count_a" "0" "user A did not receive a like notification for own action"
}

# ── Suite 6: chat.message_sent Event ──────────────────────────────────────────

suite_message_sent() {
    log_section "Suite 6: chat.message_sent → notification for receiver"

    if [ -z "$USER_A_ID" ] || [ -z "$USER_B_ID" ]; then
        skip "user IDs not available — skipping chat.message_sent suite"
        return
    fi

    # Try to get or create a conversation via chat-service (needed for resolution).
    log_test "get or create conversation for the match"
    local conv_resp
    conv_resp=$(http_post "${CHAT_URL}/api/v1/chat/conversations" \
        "{\"match_id\":${MATCH_ID:-1},\"user1_id\":${USER_A_ID},\"user2_id\":${USER_B_ID}}" \
        2>/dev/null || echo "{}")
    CONV_ID=$(echo "$conv_resp" | jq -r '.conversation.id // empty' 2>/dev/null || echo "")

    if [ -z "$CONV_ID" ]; then
        skip "could not create conversation — chat-service not available or match not found"
        return
    fi
    pass "conversation created/fetched id=${CONV_ID}"

    # Publish message.sent in chat-service format.
    local payload
    payload=$(printf '{"type":"chat.message_sent","occurred_at":"%s","payload":{"conversation_id":"%s","message_id":"msg-%d","sender_id":%s}}' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$CONV_ID" "$RUN_ID" "$USER_A_ID")

    log_test "publish chat.message_sent to RabbitMQ (A sent message to B)"
    local rmq_status
    rmq_status=$(rabbitmq_publish "chat.message_sent" "$payload")
    assert_eq "$rmq_status" "200" "RabbitMQ accepted chat.message_sent"

    wait_async "notification-service to resolve conversation and notify receiver" 4

    log_test "user B (receiver) has new_message notification"
    local resp_b
    resp_b=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_B_ID}")
    local count_b
    count_b=$(echo "$resp_b" | jq '[.notifications[]? | select(.Type=="new_message")] | length' 2>/dev/null || echo 0)
    assert_gt "$count_b" "0" "user B has at least 1 new_message notification"

    log_test "user A (sender) has NO new_message notification"
    local resp_a
    resp_a=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID}")
    local count_a
    count_a=$(echo "$resp_a" | jq '[.notifications[]? | select(.Type=="new_message")] | length' 2>/dev/null || echo 0)
    assert_eq "$count_a" "0" "user A did not receive a message notification for own message"
}

# ── Suite 7: REST API ──────────────────────────────────────────────────────────

suite_rest_api() {
    log_section "Suite 7: REST API"

    log_test "GET /api/v1/notifications/{user_id} returns 200"
    local status
    status=$(http_status "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID:-1}")
    assert_eq "$status" "200" "GET notifications returns 200"

    log_test "GET /api/v1/notifications/{user_id} response has expected fields"
    local resp
    resp=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID:-1}")
    local has_notifs
    has_notifs=$(echo "$resp" | jq 'has("notifications") and has("unread_count")' 2>/dev/null || echo "false")
    assert_eq "$has_notifs" "true" "response contains notifications and unread_count"

    log_test "GET /api/v1/notifications/0 with invalid user_id returns 200 (empty list)"
    local resp_empty
    resp_empty=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/99999999")
    local notifications
    notifications=$(echo "$resp_empty" | jq '.notifications | length' 2>/dev/null || echo "-1")
    assert_eq "$notifications" "0" "unknown user returns empty list"

    log_test "POST /api/v1/notifications/{id}/read"
    if [ -n "$NOTIF_ID" ] && [ "$NOTIF_ID" != "null" ]; then
        local read_status
        read_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
            -X POST "${NOTIFICATION_URL}/api/v1/notifications/${NOTIF_ID}/read")
        assert_eq "$read_status" "204" "mark notification ${NOTIF_ID} as read returns 204"
    else
        skip "no notification ID captured — skipping mark-as-read test"
    fi

    log_test "GET with limit and offset query params"
    local resp_paged
    resp_paged=$(http_get "${NOTIFICATION_URL}/api/v1/notifications/${USER_A_ID:-1}?limit=1&offset=0")
    local notif_count
    notif_count=$(echo "$resp_paged" | jq '.notifications | length' 2>/dev/null || echo "-1")
    [ "$notif_count" -le 1 ] \
        && pass "limit=1 returns at most 1 notification (got: ${notif_count})" \
        || fail "limit=1 returned more than 1 notification (got: ${notif_count})"
}

# ── Suite 8: Observability ─────────────────────────────────────────────────────

suite_observability() {
    log_section "Suite 8: Observability"

    log_test "GET /metrics returns 200"
    assert_eq "$(http_status "${NOTIFICATION_URL}/metrics")" "200" "/metrics endpoint is up"

    log_test "/metrics contains http_requests_total"
    local metrics
    metrics=$(http_get "${NOTIFICATION_URL}/metrics")
    echo "$metrics" | grep -q "http_requests_total" \
        && pass "http_requests_total present in /metrics" \
        || fail "http_requests_total not found in /metrics"

    log_test "/metrics contains notifications_delivered_total"
    echo "$metrics" | grep -q "notifications_delivered_total" \
        && pass "notifications_delivered_total present in /metrics" \
        || skip "notifications_delivered_total not yet in /metrics (no successful deliveries)"
}

# ── Summary ────────────────────────────────────────────────────────────────────

print_summary() {
    echo ""
    echo -e "${BOLD}${BLUE}══════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${BLUE}  Test Run Summary${NC}"
    echo -e "${BOLD}${BLUE}══════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}✔  Passed : ${PASSED}${NC}"
    echo -e "  ${RED}✘  Failed : ${FAILED}${NC}"
    echo -e "  ${YELLOW}⊘  Skipped: ${SKIPPED}${NC}"
    echo ""
    echo -e "  Total   : $((PASSED + FAILED + SKIPPED))"
    echo ""

    if [ "$FAILED" -eq 0 ]; then
        echo -e "  ${GREEN}${BOLD}✔  ALL TESTS PASSED${NC}"
        echo ""
        exit 0
    else
        echo -e "  ${RED}${BOLD}✘  ${FAILED} TEST(S) FAILED${NC}"
        echo ""
        exit 1
    fi
}

# ── Main ───────────────────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}${BLUE}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${BLUE}║   Dating Bot — Notification Service Test Suite       ║${NC}"
echo -e "${BOLD}${BLUE}╚══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  Run ID      : ${RUN_ID}"
echo -e "  TG User A   : ${TG_A}"
echo -e "  TG User B   : ${TG_B}"
echo -e "  Notification: ${NOTIFICATION_URL}"
echo -e "  Gateway Int : ${GATEWAY_INTERNAL_URL}"
echo -e "  RabbitMQ    : ${RABBITMQ_API}"

check_dependencies
suite_health
suite_gateway_delivery
suite_register_users
suite_match_created
suite_interaction_liked
suite_message_sent
suite_rest_api
suite_observability
print_summary
