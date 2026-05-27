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

// Package gcal manages the Google Calendar integration: token storage, OAuth
// flow helpers, and the server-side Calendar API proxy. Tokens are kept in a
// dedicated SQLite database (separate from the main Vikunja DB) and encrypted
// at rest using AES-256-GCM with a key that never enters either database.
package gcal

import (
	"fmt"
	"os"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/log"

	"xorm.io/xorm"
	"xorm.io/xorm/names"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

var (
	tokenDB   *xorm.Engine
	tokenOnce sync.Once
)

// UserToken is the persisted Google OAuth credential for a single Vikunja user.
// The refresh token is encrypted; access tokens are never stored.
type UserToken struct {
	UserID                int64     `xorm:"pk"`
	EncryptedRefreshToken string    `xorm:"text notnull"`
	LinkedAt              time.Time `xorm:"notnull"`
	ShowInVikunja         bool      `xorm:"notnull default false"`
	ShowVikunjaInGoogle   bool      `xorm:"notnull default false"`
}

// TableName satisfies the xorm TableName interface.
func (UserToken) TableName() string { return "user_tokens" }

// InitDB opens (creating if necessary) the dedicated Google-token SQLite
// database and runs a schema sync. It is safe to call multiple times; only the
// first call does real work.
func InitDB() error {
	var initErr error
	tokenOnce.Do(func() {
		initErr = initDB()
	})
	return initErr
}

func initDB() error {
	path := config.GoogleCalendarDBPath.GetString()

	engine, err := xorm.NewEngine("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("gcal: open token database: %w", err)
	}

	engine.SetMapper(names.GonicMapper{})
	engine.SetLogger(log.NewXormLogger(false, "stdout", "WARNING", "text"))

	if err = engine.Sync2(new(UserToken)); err != nil {
		return fmt.Errorf("gcal: sync token schema: %w", err)
	}

	// Restrict file permissions so only the Vikunja process can read the DB.
	if err = os.Chmod(path, 0600); err != nil {
		log.Warningf("gcal: could not restrict token DB permissions: %v", err)
	}

	tokenDB = engine
	log.Infof("Google Calendar token database initialised at %s", path)
	return nil
}

// db returns the token engine, panicking if InitDB was not called first.
func db() *xorm.Engine {
	if tokenDB == nil {
		panic("gcal: token database not initialised — call gcal.InitDB() at startup")
	}
	return tokenDB
}

// GetToken returns the stored token for userID, or (nil, nil) when no token exists.
func GetToken(userID int64) (*UserToken, error) {
	t := &UserToken{}
	found, err := db().Where("user_id = ?", userID).Get(t)
	if err != nil {
		return nil, fmt.Errorf("gcal: get token: %w", err)
	}
	if !found {
		return nil, nil
	}
	return t, nil
}

// IsLinked reports whether userID has a stored Google credential.
func IsLinked(userID int64) (bool, error) {
	t, err := GetToken(userID)
	return t != nil, err
}

// SaveToken encrypts refreshToken and upserts the row for userID.
func SaveToken(userID int64, refreshToken string, settings *UserToken) error {
	enc, err := encryptToken(refreshToken)
	if err != nil {
		return err
	}

	row := &UserToken{
		UserID:                userID,
		EncryptedRefreshToken: enc,
		LinkedAt:              time.Now(),
	}
	if settings != nil {
		row.ShowInVikunja = settings.ShowInVikunja
		row.ShowVikunjaInGoogle = settings.ShowVikunjaInGoogle
	}

	existing, err := GetToken(userID)
	if err != nil {
		return err
	}

	if existing == nil {
		_, err = db().Insert(row)
	} else {
		_, err = db().Where("user_id = ?", userID).Update(row)
	}
	return err
}

// GetRefreshToken decrypts and returns the refresh token for userID.
// Returns ("", nil) when the user has no linked account.
func GetRefreshToken(userID int64) (string, error) {
	t, err := GetToken(userID)
	if err != nil || t == nil {
		return "", err
	}
	return decryptToken(t.EncryptedRefreshToken)
}

// UpdateSettings persists show_in_vikunja / show_vikunja_in_google for userID.
func UpdateSettings(userID int64, showInVikunja, showVikunjaInGoogle bool) error {
	_, err := db().Where("user_id = ?", userID).
		Cols("show_in_vikunja", "show_vikunja_in_google").
		Update(&UserToken{
			ShowInVikunja:       showInVikunja,
			ShowVikunjaInGoogle: showVikunjaInGoogle,
		})
	return err
}

// DeleteToken removes the stored credential for userID (call after revoking
// the token with Google).
func DeleteToken(userID int64) error {
	_, err := db().Where("user_id = ?", userID).Delete(new(UserToken))
	return err
}
