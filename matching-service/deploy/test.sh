#!/usr/bin/env bash
set -e

# ============================================================
# Matching Service — Tests (via User Profile Service)
# ============================================================
# All user_ids come from User Profile Service registration,
# so the validation interceptor can verify them.
# ============================================================

USER_SVC="localhost:50051"
MATCH_SVC="localhost:50052"
HTTP_USER="localhost:8080"
HTTP_MATCH="localhost:8081"

echo "========================================="
echo "  Matching Service — Test Scripts"
echo "  (users from User Profile Service)"
echo "========================================="

section() {
  echo ""
  echo "========================================="
  echo "  $1"
  echo "========================================="
}

# ============================================================
# Register test users via User Profile Service
# ============================================================
section "Registering test users via User Profile Service"

# User A (Alice)
ALICE_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 500001,
  "username": "alice",
  "first_name": "Alice",
  "last_name": "Wonder"
}' "$USER_SVC" user_profile.v1.UserService/RegisterUser)
ALICE_ID=$(echo "$ALICE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Alice user_id: $ALICE_ID"

# User B (Bob)
BOB_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 500002,
  "username": "bob",
  "first_name": "Bob",
  "last_name": "Builder"
}' "$USER_SVC" user_profile.v1.UserService/RegisterUser)
BOB_ID=$(echo "$BOB_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Bob user_id: $BOB_ID"

# User C (Charlie)
CHARLIE_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": 500003,
  "username": "charlie",
  "first_name": "Charlie",
  "last_name": "Chaplin"
}' "$USER_SVC" user_profile.v1.UserService/RegisterUser)
CHARLIE_ID=$(echo "$CHARLIE_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
echo "  → Charlie user_id: $CHARLIE_ID"

# ============================================================
# gRPC tests
# ============================================================
section "gRPC Tests (grpcurl)"

echo ""
echo "Listing services..."
grpcurl -plaintext "$MATCH_SVC" list

echo ""
echo "Listing MatchingService methods..."
grpcurl -plaintext "$MATCH_SVC" list matching.v1.MatchingService

# 1. Like (Alice → Bob)
section "1. Like (Alice → Bob) — no match yet"

ALICE_LIKES_BOB=$(grpcurl -plaintext -d '{
  "from_user_id": '$ALICE_ID',
  "to_user_id": '$BOB_ID'
}' "$MATCH_SVC" matching.v1.MatchingService/Like)
echo "$ALICE_LIKES_BOB"
IS_MATCH=$(echo "$ALICE_LIKES_BOB" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: False)"

# 2. Like (Bob → Alice) → MUTUAL MATCH
section "2. Like (Bob → Alice) → MUTUAL MATCH"

BOB_LIKES_ALICE=$(grpcurl -plaintext -d '{
  "from_user_id": '$BOB_ID',
  "to_user_id": '$ALICE_ID'
}' "$MATCH_SVC" matching.v1.MatchingService/Like)
echo "$BOB_LIKES_ALICE"
IS_MATCH=$(echo "$BOB_LIKES_ALICE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('isMatch', False))")
echo "  → is_match: $IS_MATCH (expected: True)"
MATCH_ID=$(echo "$BOB_LIKES_ALICE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('match',{}).get('id',''))" 2>/dev/null || echo "")
echo "  → match_id: $MATCH_ID"

# 3. HasMatched (Alice, Bob)
section "3. HasMatched (Alice, Bob) — should be true"

grpcurl -plaintext -d '{"user1_id": '$ALICE_ID', "user2_id": '$BOB_ID'}' "$MATCH_SVC" matching.v1.MatchingService/HasMatched

# 4. HasMatched (Alice, Charlie) — no match
section "4. HasMatched (Alice, Charlie) — no match"

grpcurl -plaintext -d '{"user1_id": '$ALICE_ID', "user2_id": '$CHARLIE_ID'}' "$MATCH_SVC" matching.v1.MatchingService/HasMatched

# 5. GetMatch
section "5. GetMatch"

if [ -n "$MATCH_ID" ] && [ "$MATCH_ID" != "" ]; then
  grpcurl -plaintext -d '{"match_id": '$MATCH_ID'}' "$MATCH_SVC" matching.v1.MatchingService/GetMatch
else
  echo "  → Skipping (no match created)"
fi

# 6. GetUserMatches
section "6. GetUserMatches (Alice)"

grpcurl -plaintext -d '{"user_id": '$ALICE_ID', "page": 1, "page_size": 10}' "$MATCH_SVC" matching.v1.MatchingService/GetUserMatches

# 7. Pass (Alice → Charlie)
section "7. Pass (Alice → Charlie)"

grpcurl -plaintext -d '{
  "from_user_id": '$ALICE_ID',
  "to_user_id": '$CHARLIE_ID'
}' "$MATCH_SVC" matching.v1.MatchingService/Pass > /dev/null
echo "  → Alice passed Charlie"

# 8. GetInteractionHistory
section "8. GetInteractionHistory (Alice)"

grpcurl -plaintext -d '{"user_id": '$ALICE_ID', "page": 1, "page_size": 10}' "$MATCH_SVC" matching.v1.MatchingService/GetInteractionHistory

# 9. MarkConversationStarted
section "9. MarkConversationStarted"

if [ -n "$MATCH_ID" ] && [ "$MATCH_ID" != "" ]; then
  grpcurl -plaintext -d '{"match_id": '$MATCH_ID'}' "$MATCH_SVC" matching.v1.MatchingService/MarkConversationStarted > /dev/null
  echo "  → Conversation marked"
else
  echo "  → Skipping (no match)"
fi

# 10. UndoLike
section "10. UndoLike (Charlie → Alice)"

# First create a like
grpcurl -plaintext -d '{"from_user_id": '$CHARLIE_ID', "to_user_id": '$ALICE_ID'}' "$MATCH_SVC" matching.v1.MatchingService/Like > /dev/null
echo "  → Charlie liked Alice"
grpcurl -plaintext -d '{"from_user_id": '$CHARLIE_ID', "to_user_id": '$ALICE_ID'}' "$MATCH_SVC" matching.v1.MatchingService/UndoLike
echo "  → Charlie's like undone"

# ============================================================
# HTTP tests
# ============================================================
section "HTTP Tests (curl)"

# 1. Like (HTTP)
section "1. Like (HTTP POST) — Charlie → Bob"

curl -s -X POST "http://$HTTP_MATCH/api/v1/matching/like" \
  -H "Content-Type: application/json" \
  -d '{"from_user_id": '$CHARLIE_ID', "to_user_id": '$BOB_ID'}' | python3 -m json.tool 2>/dev/null || echo "(raw)"

# 2. HasMatched (HTTP)
section "2. HasMatched (HTTP GET) — Charlie, Bob"

curl -s "http://$HTTP_MATCH/api/v1/matching/check/$CHARLIE_ID/$BOB_ID" | python3 -m json.tool 2>/dev/null || echo "(raw)"

# 3. GetUserMatches (HTTP)
section "3. GetUserMatches (HTTP GET) — Bob"

curl -s "http://$HTTP_MATCH/api/v1/matches/user/$BOB_ID?page=1&page_size=10" | python3 -m json.tool 2>/dev/null || echo "(raw)"

# 4. GetInteractionHistory (HTTP)
section "4. GetInteractionHistory (HTTP GET) — Alice"

curl -s "http://$HTTP_MATCH/api/v1/matching/history/$ALICE_ID?page=1&page_size=10" | python3 -m json.tool 2>/dev/null || echo "(raw)"

echo ""
echo "========================================="
echo "  All tests completed!"
echo "  Users: Alice($ALICE_ID), Bob($BOB_ID), Charlie($CHARLIE_ID)"
echo "========================================="
