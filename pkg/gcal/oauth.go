// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package gcal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/modules/keyvalue"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// CalendarReadOnlyScope is the minimum scope needed to read events.
	CalendarReadOnlyScope = "https://www.googleapis.com/auth/calendar.readonly"

	// stateKeyPrefix namespaces OAuth state tokens in the keyvalue store.
	stateKeyPrefix = "gcal:oauth:state:"

	// stateTTL is how long a state token remains valid. Google typically
	// redirects back within seconds; 10 minutes is generous.
	stateTTL = 10 * time.Minute
)

// oauthConfig builds the oauth2.Config from the current Vikunja config.
func oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.GoogleCalendarClientID.GetString(),
		ClientSecret: config.GoogleCalendarClientSecret.GetString(),
		RedirectURL:  config.GoogleCalendarRedirectURL.GetString(),
		Scopes:       []string{CalendarReadOnlyScope},
		Endpoint:     google.Endpoint,
	}
}

// GenerateAuthURL creates a one-time CSRF state token for userID, stores it in
// the keyvalue store with a short TTL, and returns the Google OAuth URL.
func GenerateAuthURL(userID int64) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("gcal: generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(raw)

	if err := keyvalue.Put(stateKeyPrefix+state, userID); err != nil {
		return "", fmt.Errorf("gcal: store state: %w", err)
	}

	// Schedule expiry by storing a companion key with the TTL sentinel.
	// keyvalue doesn't support TTL natively, so we record the expiry time and
	// check it in ConsumeState.
	expiresAt := time.Now().Add(stateTTL).Unix()
	if err := keyvalue.Put(stateKeyPrefix+state+":exp", expiresAt); err != nil {
		return "", fmt.Errorf("gcal: store state expiry: %w", err)
	}

	url := oauthConfig().AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"), // always return refresh token
	)
	return url, nil
}

// ConsumeState validates the state token and returns the associated userID.
// The token is deleted from the store on success (single-use).
func ConsumeState(state string) (int64, error) {
	key := stateKeyPrefix + state
	expKey := key + ":exp"

	// Check expiry first.
	rawExp, exists, err := keyvalue.Get(expKey)
	if err != nil {
		return 0, fmt.Errorf("gcal: read state expiry: %w", err)
	}
	if !exists {
		return 0, errors.New("gcal: unknown or expired state token")
	}
	exp, ok := rawExp.(int64)
	if !ok {
		return 0, errors.New("gcal: malformed state expiry")
	}
	if time.Now().Unix() > exp {
		_ = keyvalue.Del(key)
		_ = keyvalue.Del(expKey)
		return 0, errors.New("gcal: state token expired")
	}

	rawID, exists, err := keyvalue.Get(key)
	if err != nil {
		return 0, fmt.Errorf("gcal: read state: %w", err)
	}
	if !exists {
		return 0, errors.New("gcal: unknown state token")
	}

	userID, ok := rawID.(int64)
	if !ok {
		return 0, errors.New("gcal: malformed state user ID")
	}

	_ = keyvalue.Del(key)
	_ = keyvalue.Del(expKey)
	return userID, nil
}

// ExchangeCode exchanges an authorization code for tokens and persists the
// encrypted refresh token for userID.
func ExchangeCode(userID int64, code string) error {
	token, err := oauthConfig().Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("gcal: exchange code: %w", err)
	}

	if token.RefreshToken == "" {
		return errors.New("gcal: Google did not return a refresh token — ensure the OAuth consent includes access_type=offline and prompt=consent")
	}

	return SaveToken(userID, token.RefreshToken, nil)
}

// FreshAccessToken returns a valid access token for userID, using the stored
// refresh token to obtain a new one if needed. The access token is never
// persisted — it exists only in memory for the duration of the request.
func FreshAccessToken(userID int64) (string, error) {
	refreshToken, err := GetRefreshToken(userID)
	if err != nil {
		return "", err
	}
	if refreshToken == "" {
		return "", errors.New("gcal: no linked Google account for this user")
	}

	src := oauthConfig().TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})
	t, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("gcal: refresh access token: %w", err)
	}

	return t.AccessToken, nil
}

// RevokeToken revokes the stored refresh token with Google's revocation endpoint
// and deletes the row from the token database.
func RevokeToken(userID int64) error {
	refreshToken, err := GetRefreshToken(userID)
	if err != nil {
		return err
	}

	if refreshToken != "" {
		// Best-effort revocation — we delete locally even if Google errors.
		cfg := oauthConfig()
		ctx := context.Background()
		_ = cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
		// Google's revocation endpoint is not part of the oauth2 package; call it directly.
		revokeURL := fmt.Sprintf("https://oauth2.googleapis.com/revoke?token=%s", refreshToken)
		resp, httpErr := cfg.Client(ctx, &oauth2.Token{RefreshToken: refreshToken}).Post(revokeURL, "application/x-www-form-urlencoded", nil)
		if httpErr == nil {
			_ = resp.Body.Close()
		}
	}

	return DeleteToken(userID)
}
