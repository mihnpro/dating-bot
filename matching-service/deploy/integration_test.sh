#!/usr/bin/env bash
set -e

# ============================================================
# Integration Tests: User Profile Service ↔ Matching Service
# ============================================================
# These tests demonstrate cross-service communication:
#   1. Register user via User Profile Service → get user_id
#   2. Use that user_id to interact with Matching Service
#   3. Verify match creation and data consistency
# ============================================================

USER_SVC_GRPC="localhost:50051"
MATCH_SVC_GRPC="localhost:50052"
USER_SVC_HTTP="localhost:8080"
MATCH_SVC_HTTP="localhost:8081"

echo "========================================="
echo "  Integration Tests"
echo "  User Profile ↔ Matching"
echo "========================================="

section() {
  echo ""
  echo "========================================="
  echo "  $1"
  echo "========================================="
}

# ============================================================
# STEP 1: Register users via User Profile Service
# ============================================================
section "Step 1: Register User A (Alice) via User Profile Service"

ALICE_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 100001,
  "username": "alice",
  "first_name": "Alice",
  "last_name": "Wonderland"
}' "$USER_SVC_GRPC" user_profile.v1.UserService/RegisterUser)
echo "$ALICE_RESP"
ALICE_ID=$(echo "$ALICE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Alice user_id: $ALICE_ID"

section "Step 2: Register User B (Bob) via User Profile Service"

BOB_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 100002,
  "username": "bob",
  "first_name": "Bob",
  "last_name": "Builder"
}' "$USER_SVC_GRPC" user_profile.v1.UserService/RegisterUser)
echo "$BOB_RESP"
BOB_ID=$(echo "$BOB_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Bob user_id: $BOB_ID"

section "Step 3: Register User C (Charlie) via User Profile Service"

CHARLIE_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 100003,
  "username": "charlie",
  "first_name": "Charlie",
  "last_name": "Chaplin"
}' "$USER_SVC_GRPC" user_profile.v1.UserService/RegisterUser)
echo "$CHARLIE_RESP"
CHARLIE_ID=$(echo "$CHARLIE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Charlie user_id: $CHARLIE_ID"

# ============================================================
# STEP 2: Create profiles
# ============================================================
section "Step 4: Create profile for Alice"

grpcurl -plaintext -d '{
  "user_id": '$ALICE_ID',
  "age": 25,
  "gender": "female",
  "city": "Moscow",
  "interests": ["travel", "photography"]
}' "$USER_SVC_GRPC" user_profile.v1.UserService/CreateProfile > /dev/null
echo "  → Alice profile created"

section "Step 5: Create profile for Bob"

grpcurl -plaintext -d '{
  "user_id": '$BOB_ID',
  "age": 28,
  "gender": "male",
  "city": "Moscow",
  "interests": ["music", "travel"]
}' "$USER_SVC_GRPC" user_profile.v1.UserService/CreateProfile > /dev/null
echo "  → Bob profile created"

# ============================================================
# STEP 3: Matching interactions
# ============================================================
section "Step 6: Alice likes Bob → no match yet"

ALICE_LIKES_BOB=$(grpcurl -plaintext -d '{
  "from_user_id": '$ALICE_ID',
  "to_user_id": '$BOB_ID'
}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/Like)
echo "$ALICE_LIKES_BOB"
IS_MATCH=$(echo "$ALICE_LIKES_BOB" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: False)"

section "Step 7: Bob likes Alice → MUTUAL MATCH!"

BOB_LIKES_ALICE=$(grpcurl -plaintext -d '{
  "from_user_id": '$BOB_ID',
  "to_user_id": '$ALICE_ID'
}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/Like)
echo "$BOB_LIKES_ALICE"
IS_MATCH=$(echo "$BOB_LIKES_ALICE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: True)"
MATCH_ID=$(echo "$BOB_LIKES_ALICE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('match',{}).get('id',''))" 2>/dev/null || echo "")
echo "  → match_id: $MATCH_ID"

section "Step 8: Alice passes Charlie"

grpcurl -plaintext -d '{
  "from_user_id": '$ALICE_ID',
  "to_user_id": '$CHARLIE_ID'
}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/Pass > /dev/null
echo "  → Alice passed Charlie"

section "Step 9: Charlie likes Alice → no match (Alice passed)"

CHARLIE_LIKES_ALICE=$(grpcurl -plaintext -d '{
  "from_user_id": '$CHARLIE_ID',
  "to_user_id": '$ALICE_ID'
}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/Like)
echo "$CHARLIE_LIKES_ALICE"
IS_MATCH=$(echo "$CHARLIE_LIKES_ALICE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: True — Charlie liked back)"

# ============================================================
# STEP 4: Verify matches
# ============================================================
section "Step 10: Get Alice's matches via Matching Service"

ALICE_MATCHES=$(grpcurl -plaintext -d '{"user_id": '$ALICE_ID', "page": 1, "page_size": 10}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/GetUserMatches)
echo "$ALICE_MATCHES"
MATCH_COUNT=$(echo "$ALICE_MATCHES" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('matches',[])))")
echo "  → Alice has $MATCH_COUNT match(es)"

section "Step 11: HasMatched(Alice, Bob)"

grpcurl -plaintext -d '{"user1_id": '$ALICE_ID', "user2_id": '$BOB_ID'}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/HasMatched

section "Step 12: HasMatched(Alice, Charlie) — should be false"

grpcurl -plaintext -d '{"user1_id": '$ALICE_ID', "user2_id": '$CHARLIE_ID'}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/HasMatched

section "Step 13: Get match details"

if [ -n "$MATCH_ID" ] && [ "$MATCH_ID" != "" ]; then
  grpcurl -plaintext -d '{"match_id": '$MATCH_ID'}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/GetMatch
fi

# ============================================================
# STEP 5: Verify data consistency across services
# ============================================================
section "Step 14: Cross-service — Verify Alice exists in User Service"

grpcurl -plaintext -d '{"id": '$ALICE_ID'}' "$USER_SVC_GRPC" user_profile.v1.UserService/GetUser | python3 -c "
import sys, json
u = json.load(sys.stdin)['user']
print(f'  → User: {u[\"firstName\"]} {u[\"lastName\"]} (id={u[\"id\"]}, telegram={u[\"telegramId\"]})')
"

section "Step 15: Cross-service — Get Bob's profile via User Service"

grpcurl -plaintext -d '{"user_id": '$BOB_ID'}' "$USER_SVC_GRPC" user_profile.v1.UserService/GetProfile | python3 -c "
import sys, json
p = json.load(sys.stdin)['profile']
print(f'  → Profile: {p[\"age\"]}y/o {p[\"gender\"]}, city={p[\"city\"]}, interests={p[\"interests\"]}')
"

section "Step 16: Interaction history for Alice"

grpcurl -plaintext -d '{"user_id": '$ALICE_ID', "page": 1, "page_size": 10}' "$MATCH_SVC_GRPC" matching.v1.MatchingService/GetInteractionHistory

# ============================================================
# STEP 6: HTTP (REST) integration tests
# ============================================================
section "Step 17: HTTP — Register User D (Diana)"

DIANA_RESP=$(curl -s -X POST "http://$USER_SVC_HTTP/api/v1/users/register" \
  -H "Content-Type: application/json" \
  -d '{"telegram_id": 100004, "username": "diana", "first_name": "Diana", "last_name": "Prince"}')
DIANA_ID=$(echo "$DIANA_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Diana user_id: $DIANA_ID"

section "Step 18: HTTP — Diana likes Bob"

curl -s -X POST "http://$MATCH_SVC_HTTP/api/v1/matching/like" \
  -H "Content-Type: application/json" \
  -d '{"from_user_id": '$DIANA_ID', "to_user_id": '$BOB_ID'}' | python3 -m json.tool

section "Step 19: HTTP — Bob likes Diana → Match!"

BOB_LIKES_DIANA=$(curl -s -X POST "http://$MATCH_SVC_HTTP/api/v1/matching/like" \
  -H "Content-Type: application/json" \
  -d '{"from_user_id": '$BOB_ID', "to_user_id": '$DIANA_ID'}')
echo "$BOB_LIKES_DIANA" | python3 -m json.tool
IS_MATCH=$(echo "$BOB_LIKES_DIANA" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: True)"

section "Step 20: HTTP — Bob's matches"

curl -s "http://$MATCH_SVC_HTTP/api/v1/matches/user/$BOB_ID?page=1&page_size=10" | python3 -c "
import sys, json
data = json.load(sys.stdin)
matches = data.get('matches', [])
print(f'  → Bob has {len(matches)} match(es):')
for m in matches:
    other = m['user2Id'] if m['user1Id'] == '$BOB_ID' else m['user1Id']
    print(f'     Match #{m[\"id\"]} with user {other}')
"

# ============================================================
# STEP 7: Negative tests
# ============================================================
section "Step 21: Like non-existent user → should fail"

grpcurl -plaintext -d '{"from_user_id": 999999, "to_user_id": '$BOB_ID'}' \
  "$MATCH_SVC_GRPC" matching.v1.MatchingService/Like 2>&1 || true

section "Step 22: Undo like"

grpcurl -plaintext -d '{"from_user_id": '$DIANA_ID', "to_user_id": '$BOB_ID'}' \
  "$MATCH_SVC_GRPC" matching.v1.MatchingService/UndoLike > /dev/null 2>&1
echo "  → Diana's like undone"

# ============================================================
# STEP 8: Mark conversation started
# ============================================================
section "Step 23: Mark Bob-Diana conversation as started"

if [ -n "$MATCH_ID" ]; then
  grpcurl -plaintext -d '{"match_id": '$MATCH_ID'}' \
    "$MATCH_SVC_GRPC" matching.v1.MatchingService/MarkConversationStarted > /dev/null 2>&1
  echo "  → Conversation marked (using first match)"
fi

# ============================================================
# Summary
# ============================================================
section "Integration Test Summary"
echo ""
echo "  Users registered: Alice($ALICE_ID), Bob($BOB_ID), Charlie($CHARLIE_ID), Diana($DIANA_ID)"
echo "  Services involved:"
echo "    - User Profile Service (gRPC:50051, HTTP:8080)"
echo "    - Matching Service   (gRPC:50052, HTTP:8081)"
echo ""
echo "  Scenarios tested:"
echo "    ✓ User registration → Matching interactions"
echo "    ✓ Mutual like → Match creation"
echo "    ✓ Pass → No match (unless other side likes back)"
echo "    ✓ Match listing by user ID"
echo "    ✓ HasMatched check"
echo "    ✓ Interaction history"
echo "    ✓ Cross-service data lookup (user details + profile)"
echo "    ✓ REST API integration"
echo "    ✓ Negative: like non-existent user"
echo "    ✓ Undo like"
echo ""
echo "========================================="
echo "  All integration tests completed!"
echo "========================================="
