package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TokenService generates and validates short-lived HMAC tokens for WebSocket auth.
// Token format: "<user_id>:<expires_unix>:<hmac_hex>"
// The token is passed in the WebSocket URL: /ws?token=<token>
type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (t *TokenService) Generate(userID int64) string {
	expires := time.Now().Add(t.ttl).Unix()
	msg := fmt.Sprintf("%d:%d", userID, expires)
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", msg, sig)
}

// Validate returns the userID if the token is valid and not expired.
func (t *TokenService) Validate(token string) (int64, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid token format")
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user_id in token")
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expiry in token")
	}
	if time.Now().Unix() > expires {
		return 0, fmt.Errorf("token expired")
	}

	msg := fmt.Sprintf("%d:%d", userID, expires)
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return 0, fmt.Errorf("invalid token signature")
	}
	return userID, nil
}
