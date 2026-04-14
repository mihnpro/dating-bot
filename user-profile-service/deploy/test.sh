#!/usr/bin/env bash
set -e

GRPC_ADDR="localhost:50051"
HTTP_ADDR="localhost:8080"

# Unique IDs per run to avoid duplicate key errors
GRPC_TELEGRAM_ID=$((RANDOM % 900000000 + 100000000))
HTTP_TELEGRAM_ID=$((RANDOM % 900000000 + 100000000))

echo "========================================="
echo "  User Profile Service — Test Scripts"
echo "========================================="
echo ""

# --- Helper ---
section() {
  echo ""
  echo "========================================="
  echo "  $1"
  echo "========================================="
}

# =============================================
# gRPC tests (grpcurl)
# =============================================
section "gRPC Tests (grpcurl)"

echo ""
echo "Listing available services..."
grpcurl -plaintext "$GRPC_ADDR" list

echo ""
echo "Listing UserService methods..."
grpcurl -plaintext "$GRPC_ADDR" list user_profile.v1.UserService

# 1. RegisterUser
section "1. RegisterUser"
REGISTER_RESP=$(grpcurl -plaintext -d '{
  "telegram_id": '$GRPC_TELEGRAM_ID',
  "username": "testuser",
  "first_name": "John",
  "last_name": "Doe"
}' "$GRPC_ADDR" user_profile.v1.UserService/RegisterUser)
echo "$REGISTER_RESP"
USER_ID=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['user']['id'])" 2>/dev/null)
echo "  → User ID: $USER_ID"

# 2. GetUser
section "2. GetUser (by ID)"
grpcurl -plaintext -d '{"id": '$USER_ID'}' "$GRPC_ADDR" user_profile.v1.UserService/GetUser

# 3. GetUserByTelegramID
section "3. GetUserByTelegramID"
grpcurl -plaintext -d '{"telegram_id": '$GRPC_TELEGRAM_ID'}' "$GRPC_ADDR" user_profile.v1.UserService/GetUserByTelegramID

# 4. UpdateUser
section "4. UpdateUser"
grpcurl -plaintext -d '{
  "id": '$USER_ID',
  "first_name": "Johnny"
}' "$GRPC_ADDR" user_profile.v1.UserService/UpdateUser

# 5. ListUsers
section "5. ListUsers"
grpcurl -plaintext -d '{"page": 1, "page_size": 10}' "$GRPC_ADDR" user_profile.v1.UserService/ListUsers

# 6. CreateProfile
section "6. CreateProfile"
PROFILE_RESP=$(grpcurl -plaintext -d '{
  "user_id": '$USER_ID',
  "age": 25,
  "gender": "male",
  "city": "Moscow",
  "interests": ["travel", "music", "sports"]
}' "$GRPC_ADDR" user_profile.v1.UserService/CreateProfile)
echo "$PROFILE_RESP"

# 7. GetProfile
section "7. GetProfile"
grpcurl -plaintext -d '{"user_id": '$USER_ID'}' "$GRPC_ADDR" user_profile.v1.UserService/GetProfile

# 8. UpdateProfile
section "8. UpdateProfile"
grpcurl -plaintext -d '{
  "user_id": '$USER_ID',
  "age": 26,
  "city": "Saint Petersburg"
}' "$GRPC_ADDR" user_profile.v1.UserService/UpdateProfile

# 9. ListProfiles
section "9. ListProfiles"
grpcurl -plaintext -d '{"page": 1, "page_size": 10}' "$GRPC_ADDR" user_profile.v1.UserService/ListProfiles

# 10. DeleteProfile
section "10. DeleteProfile"
grpcurl -plaintext -d '{"user_id": '$USER_ID'}' "$GRPC_ADDR" user_profile.v1.UserService/DeleteProfile

# 11. DeleteUser
section "11. DeleteUser"
grpcurl -plaintext -d '{"id": '$USER_ID'}' "$GRPC_ADDR" user_profile.v1.UserService/DeleteUser

# =============================================
# HTTP tests (curl via gRPC-Gateway)
# =============================================
section "HTTP Tests (curl — gRPC Gateway)"

# 1. RegisterUser (POST)
section "1. RegisterUser (HTTP POST)"
REGISTER_HTTP=$(curl -s -X POST "http://$HTTP_ADDR/api/v1/users/register" \
  -H "Content-Type: application/json" \
  -d '{"telegram_id": '$HTTP_TELEGRAM_ID', "username": "httpuser", "first_name": "Jane", "last_name": "Smith"}')
echo "$REGISTER_HTTP" | python3 -m json.tool 2>/dev/null || echo "$REGISTER_HTTP"
HTTP_USER_ID=$(echo "$REGISTER_HTTP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])" 2>/dev/null || echo "")
echo "  → User ID: $HTTP_USER_ID"

# 2. GetUser (GET)
section "2. GetUser (HTTP GET)"
curl -s "http://$HTTP_ADDR/api/v1/users/$HTTP_USER_ID" | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 3. GetUserByTelegramID (GET)
section "3. GetUserByTelegramID (HTTP GET)"
curl -s "http://$HTTP_ADDR/api/v1/users/telegram/$HTTP_TELEGRAM_ID" | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 4. UpdateUser (PATCH)
section "4. UpdateUser (HTTP PATCH)"
curl -s -X PATCH "http://$HTTP_ADDR/api/v1/users/$HTTP_USER_ID" \
  -H "Content-Type: application/json" \
  -d '{"first_name": "Janet"}' | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 5. ListUsers (GET)
section "5. ListUsers (HTTP GET)"
curl -s "http://$HTTP_ADDR/api/v1/users?page=1&page_size=10" | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 6. CreateProfile (POST)
section "6. CreateProfile (HTTP POST)"
curl -s -X POST "http://$HTTP_ADDR/api/v1/profiles" \
  -H "Content-Type: application/json" \
  -d '{"user_id": '$HTTP_USER_ID', "age": 28, "gender": "female", "city": "Kazan", "interests": ["reading", "cooking"]}' \
  | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 7. GetProfile (GET)
section "7. GetProfile (HTTP GET)"
curl -s "http://$HTTP_ADDR/api/v1/profiles/$HTTP_USER_ID" | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 8. UpdateProfile (PATCH)
section "8. UpdateProfile (HTTP PATCH)"
curl -s -X PATCH "http://$HTTP_ADDR/api/v1/profiles/$HTTP_USER_ID" \
  -H "Content-Type: application/json" \
  -d '{"age": 29, "city": "Moscow"}' | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 9. ListProfiles (GET)
section "9. ListProfiles (HTTP GET)"
curl -s "http://$HTTP_ADDR/api/v1/profiles?page=1&page_size=10" | python3 -m json.tool 2>/dev/null || echo "(raw response)"

# 10. DeleteProfile (DELETE)
section "10. DeleteProfile (HTTP DELETE)"
curl -s -X DELETE "http://$HTTP_ADDR/api/v1/profiles/$HTTP_USER_ID" -w "\n  HTTP Status: %{http_code}\n"

# 11. DeleteUser (DELETE)
section "11. DeleteUser (HTTP DELETE)"
curl -s -X DELETE "http://$HTTP_ADDR/api/v1/users/$HTTP_USER_ID" -w "\n  HTTP Status: %{http_code}\n"

echo ""
echo "========================================="
echo "  All tests completed!"
echo "========================================="
