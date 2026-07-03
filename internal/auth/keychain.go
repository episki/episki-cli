package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "episki-cli"
	keychainAccount = "default"
)

// Session is the Supabase Auth state stored in the OS keychain after
// `episki auth login`.
type Session struct {
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token,omitempty"`
	AccessExpiresAt time.Time `json:"access_expires_at,omitempty"`
	Email           string    `json:"email,omitempty"`
	UserID          string    `json:"user_id,omitempty"`
}

// IsExpired reports whether the access token has expired (or is within the
// given skew of expiring). Use a small skew (~30-60s) to avoid sending a
// soon-to-expire token in flight.
func (s *Session) IsExpired(skew time.Duration) bool {
	if s == nil || s.AccessExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(skew).After(s.AccessExpiresAt)
}

// LoadSession returns the stored session, or (nil, nil) if none exists.
func LoadSession() (*Session, error) {
	raw, err := keyring.Get(keychainService, keychainAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("read keychain: %w", err)
	}
	var s Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &s, nil
}

// SaveSession persists the session to the OS keychain.
func SaveSession(s *Session) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := keyring.Set(keychainService, keychainAccount, string(b)); err != nil {
		return fmt.Errorf("write keychain: %w", err)
	}
	return nil
}

// DeleteSession removes the stored session, succeeding if there's nothing to remove.
func DeleteSession() error {
	if err := keyring.Delete(keychainService, keychainAccount); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete keychain entry: %w", err)
	}
	return nil
}
