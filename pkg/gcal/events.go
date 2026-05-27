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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.vikunja.io/api/pkg/log"

	"golang.org/x/oauth2"
)

const (
	calendarListURL = "https://www.googleapis.com/calendar/v3/users/me/calendarList?minAccessRole=reader"
	eventsURLFmt    = "https://www.googleapis.com/calendar/v3/calendars/%s/events"
	maxEventsPerCal = 250
)

// Event is a single Google Calendar event returned to callers.
type Event struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	AllDay       bool      `json:"all_day"`
	CalendarName string    `json:"calendar_name"`
}

// — internal Google API response shapes —

type gcalDateTime struct {
	DateTime string `json:"dateTime"` // RFC3339 for timed events
	Date     string `json:"date"`     // YYYY-MM-DD for all-day events
	TimeZone string `json:"timeZone"`
}

type gcalEvent struct {
	ID      string       `json:"id"`
	Summary string       `json:"summary"`
	Start   gcalDateTime `json:"start"`
	End     gcalDateTime `json:"end"`
	Status  string       `json:"status"` // "confirmed", "tentative", "cancelled"
}

type gcalEventsResponse struct {
	Items         []gcalEvent `json:"items"`
	NextPageToken string      `json:"nextPageToken"`
}

type gcalCalendar struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Primary bool   `json:"primary"`
}

type gcalCalendarList struct {
	Items []gcalCalendar `json:"items"`
}

// FetchEventsForMonth returns all Google Calendar events for userID that fall
// within the given month (year/month in the user's local time). Events are
// fetched server-side; no token is returned to callers.
func FetchEventsForMonth(userID int64, year int, month time.Month) ([]Event, error) {
	refreshToken, err := GetRefreshToken(userID)
	if err != nil {
		return nil, err
	}
	if refreshToken == "" {
		return nil, nil
	}

	cfg := oauthConfig()
	ctx := context.Background()
	client := cfg.Client(ctx, &oauth2.Token{RefreshToken: refreshToken})

	// Compute the inclusive [start, end) window for the requested month.
	loc := time.UTC
	start := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	calendars, err := listCalendars(client)
	if err != nil {
		return nil, err
	}

	var all []Event
	for _, cal := range calendars {
		events, err := fetchCalendarEvents(client, cal, start, end)
		if err != nil {
			// Log but continue — a single bad calendar shouldn't blank the whole view.
			log.Errorf("gcal: fetch events for calendar %q: %v", cal.ID, err)
			continue
		}
		all = append(all, events...)
	}
	return all, nil
}

func listCalendars(client *http.Client) ([]gcalCalendar, error) {
	resp, err := client.Get(calendarListURL)
	if err != nil {
		return nil, fmt.Errorf("gcal: list calendars: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcal: list calendars: unexpected status %d", resp.StatusCode)
	}

	var list gcalCalendarList
	if err = json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("gcal: decode calendar list: %w", err)
	}
	return list.Items, nil
}

func fetchCalendarEvents(client *http.Client, cal gcalCalendar, start, end time.Time) ([]Event, error) {
	url := fmt.Sprintf(
		"%s?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime&maxResults=%d",
		fmt.Sprintf(eventsURLFmt, cal.ID),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		maxEventsPerCal,
	)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch events: unexpected status %d", resp.StatusCode)
	}

	var page gcalEventsResponse
	if err = json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}

	var events []Event
	for _, raw := range page.Items {
		if raw.Status == "cancelled" {
			continue
		}
		ev, err := toEvent(raw, cal.Summary)
		if err != nil {
			log.Warningf("gcal: skip unparseable event %q: %v", raw.ID, err)
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func toEvent(raw gcalEvent, calendarName string) (Event, error) {
	ev := Event{
		ID:           raw.ID,
		Title:        raw.Summary,
		CalendarName: calendarName,
	}

	if raw.Start.DateTime != "" {
		// Timed event
		start, err := time.Parse(time.RFC3339, raw.Start.DateTime)
		if err != nil {
			return Event{}, fmt.Errorf("parse start: %w", err)
		}
		end, err := time.Parse(time.RFC3339, raw.End.DateTime)
		if err != nil {
			return Event{}, fmt.Errorf("parse end: %w", err)
		}
		ev.Start = start
		ev.End = end
	} else {
		// All-day event — treat as midnight UTC on those dates
		ev.AllDay = true
		start, err := time.Parse("2006-01-02", raw.Start.Date)
		if err != nil {
			return Event{}, fmt.Errorf("parse all-day start: %w", err)
		}
		end, err := time.Parse("2006-01-02", raw.End.Date)
		if err != nil {
			return Event{}, fmt.Errorf("parse all-day end: %w", err)
		}
		ev.Start = start
		ev.End = end
	}

	return ev, nil
}
