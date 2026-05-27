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

package v1

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/gcal"
	"code.vikunja.io/api/pkg/models"
	user2 "code.vikunja.io/api/pkg/user"
)

// GoogleCalendarStatus is the response body for GET /user/settings/google.
type GoogleCalendarStatus struct {
	// Whether the Google Calendar feature is enabled on this instance.
	Enabled bool `json:"enabled"`
	// Whether the current user has a linked Google account.
	Linked bool `json:"linked"`
	// If linked, when the account was first connected.
	LinkedAt interface{} `json:"linked_at,omitempty"`
	// Show Google Calendar events in Vikunja calendar views.
	ShowInVikunja bool `json:"show_in_vikunja"`
	// Show Vikunja tasks in Google Calendar (not yet implemented).
	ShowVikunjaInGoogle bool `json:"show_vikunja_in_google"`
}

// GoogleCalendarAuthURL is the response body for GET /user/settings/google/auth-url.
type GoogleCalendarAuthURL struct {
	// The Google OAuth authorization URL. Redirect the user here.
	URL string `json:"url"`
}

// GoogleCalendarSettings is the request body for PATCH /user/settings/google.
type GoogleCalendarSettings struct {
	ShowInVikunja       bool `json:"show_in_vikunja"`
	ShowVikunjaInGoogle bool `json:"show_vikunja_in_google"`
}

// GetGoogleCalendarStatus returns the current user's Google Calendar link status.
// @Summary Get Google Calendar link status
// @Description Returns whether the current user has a linked Google account and their preferences.
// @tags user
// @Produce json
// @Security JWTKeyAuth
// @Success 200 {object} GoogleCalendarStatus
// @Failure 500 {object} models.Message "Internal server error."
// @Router /user/settings/google [get]
func GetGoogleCalendarStatus(c *echo.Context) error {
	status := &GoogleCalendarStatus{
		Enabled: config.GoogleCalendarEnable.GetBool(),
	}
	if !status.Enabled {
		return c.JSON(http.StatusOK, status)
	}

	u, err := user2.GetCurrentUser(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	token, err := gcal.GetToken(u.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	if token != nil {
		status.Linked = true
		status.LinkedAt = token.LinkedAt
		status.ShowInVikunja = token.ShowInVikunja
		status.ShowVikunjaInGoogle = token.ShowVikunjaInGoogle
	}

	return c.JSON(http.StatusOK, status)
}

// GetGoogleCalendarAuthURL returns a Google OAuth authorization URL for the current user.
// @Summary Get Google OAuth authorization URL
// @Description Generates a one-time OAuth state and returns the Google authorization URL.
// @tags user
// @Produce json
// @Security JWTKeyAuth
// @Success 200 {object} GoogleCalendarAuthURL
// @Failure 500 {object} models.Message "Internal server error."
// @Router /user/settings/google/auth-url [get]
func GetGoogleCalendarAuthURL(c *echo.Context) error {
	if !config.GoogleCalendarEnable.GetBool() {
		return echo.NewHTTPError(http.StatusNotFound, "Google Calendar integration is not enabled.")
	}

	u, err := user2.GetCurrentUser(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	url, err := gcal.GenerateAuthURL(u.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	return c.JSON(http.StatusOK, &GoogleCalendarAuthURL{URL: url})
}

// HandleGoogleCalendarCallback handles the OAuth callback from Google.
// This route is unauthenticated — the user identity is carried in the state token.
// @Summary Google OAuth callback
// @Description Exchanges the authorization code for tokens, stores them, and redirects to the settings page.
// @tags user
// @Param code query string true "Authorization code from Google"
// @Param state query string true "CSRF state token"
// @Success 302 "Redirects to /user/settings/google?linked=true"
// @Failure 400 {object} models.Message "Invalid state or code."
// @Failure 500 {object} models.Message "Internal server error."
// @Router /user/settings/google/callback [get]
func HandleGoogleCalendarCallback(c *echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")

	if state == "" || code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing state or code parameter.")
	}

	userID, err := gcal.ConsumeState(state)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid or expired state token.").Wrap(err)
	}

	if err = gcal.ExchangeCode(userID, code); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to exchange authorization code.").Wrap(err)
	}

	frontendURL := config.ServicePublicURL.GetString()
	return c.Redirect(http.StatusFound, frontendURL+"user/settings/google?linked=true")
}

// UpdateGoogleCalendarSettings updates show_in_vikunja / show_vikunja_in_google for the current user.
// @Summary Update Google Calendar settings
// @Description Toggles whether Google Calendar events appear in Vikunja and/or Vikunja tasks appear in Google Calendar.
// @tags user
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param settings body GoogleCalendarSettings true "Settings"
// @Success 200 {object} models.Message
// @Failure 400 {object} models.Message "Not linked."
// @Failure 500 {object} models.Message "Internal server error."
// @Router /user/settings/google [patch]
func UpdateGoogleCalendarSettings(c *echo.Context) error {
	if !config.GoogleCalendarEnable.GetBool() {
		return echo.NewHTTPError(http.StatusNotFound, "Google Calendar integration is not enabled.")
	}

	u, err := user2.GetCurrentUser(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	linked, err := gcal.IsLinked(u.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}
	if !linked {
		return echo.NewHTTPError(http.StatusBadRequest, "No linked Google account.")
	}

	var settings GoogleCalendarSettings
	if err = c.Bind(&settings); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error()).Wrap(err)
	}

	if err = gcal.UpdateSettings(u.ID, settings.ShowInVikunja, settings.ShowVikunjaInGoogle); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	return c.JSON(http.StatusOK, &models.Message{Message: "Google Calendar settings updated."})
}

// UnlinkGoogleCalendar revokes and removes the current user's Google token.
// @Summary Unlink Google Calendar
// @Description Revokes the stored Google OAuth token and removes the link.
// @tags user
// @Produce json
// @Security JWTKeyAuth
// @Success 200 {object} models.Message
// @Failure 500 {object} models.Message "Internal server error."
// @Router /user/settings/google [delete]
func UnlinkGoogleCalendar(c *echo.Context) error {
	if !config.GoogleCalendarEnable.GetBool() {
		return echo.NewHTTPError(http.StatusNotFound, "Google Calendar integration is not enabled.")
	}

	u, err := user2.GetCurrentUser(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	if err = gcal.RevokeToken(u.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error()).Wrap(err)
	}

	return c.JSON(http.StatusOK, &models.Message{Message: "Google Calendar unlinked."})
}
