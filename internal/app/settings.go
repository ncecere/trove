package app

import (
	"net/http"
	"strconv"
	"strings"
)

type settingsJSON struct {
	TMDBAPIKeySet       bool `json:"tmdbApiKeySet"`
	OpenLibraryEnabled  bool `json:"openLibraryEnabled"`
	ScanIntervalMinutes int  `json:"scanIntervalMinutes"`
}

func toSettingsJSON(s Settings) settingsJSON {
	return settingsJSON{TMDBAPIKeySet: s.TMDBAPIKey != "", OpenLibraryEnabled: s.OpenLibraryEnabled, ScanIntervalMinutes: s.ScanIntervalMinutes}
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toSettingsJSON(a.currentSettings()))
}

func (a *App) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TMDBAPIKey          *string `json:"tmdbApiKey"`
		OpenLibraryEnabled  *bool   `json:"openLibraryEnabled"`
		ScanIntervalMinutes *int    `json:"scanIntervalMinutes"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	kv := map[string]string{}
	if body.TMDBAPIKey != nil {
		k := strings.TrimSpace(*body.TMDBAPIKey)
		if len(k) > 512 {
			badRequest(w, "tmdbApiKey is too long")
			return
		}
		kv[settingTMDBKey] = k
	}
	if body.OpenLibraryEnabled != nil {
		kv[settingOpenLibrary] = "0"
		if *body.OpenLibraryEnabled {
			kv[settingOpenLibrary] = "1"
		}
	}
	if body.ScanIntervalMinutes != nil {
		n := *body.ScanIntervalMinutes
		if n < 0 || n > 7*24*60 {
			badRequest(w, "scanIntervalMinutes must be between 0 and 10080")
			return
		}
		kv[settingScanInterval] = strconv.Itoa(n)
	}
	before := a.currentSettings()
	if err := a.store.PutSettings(r.Context(), kv); err != nil {
		a.internalError(w, r, err)
		return
	}
	if err := a.loadSettings(r.Context()); err != nil {
		a.internalError(w, r, err)
		return
	}
	after := a.currentSettings()
	// New metadata sources: previously unmatched items get another chance.
	if (after.TMDBAPIKey != before.TMDBAPIKey && after.TMDBAPIKey != "") || (after.OpenLibraryEnabled && !before.OpenLibraryEnabled) {
		if err := a.store.ResetNotFound(r.Context()); err != nil {
			a.log.Warn("reset metadata state", "err", err)
		}
		if a.deps.Providers != nil {
			if libs, err := a.store.ListLibraries(r.Context()); err == nil {
				for _, l := range libs {
					a.scans.trigger(l.ID, false)
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, toSettingsJSON(after))
}

type activityJSON struct {
	ID        int64   `json:"id"`
	At        string  `json:"at"`
	UserID    *int64  `json:"userId"`
	Username  string  `json:"username"`
	Action    string  `json:"action"`
	ItemID    *int64  `json:"itemId"`
	ItemTitle *string `json:"itemTitle"`
	Detail    *string `json:"detail"`
	Size      *int64  `json:"size"`
}

func (a *App) handleActivity(w http.ResponseWriter, r *http.Request) {
	page, limit, msg := parsePaging(r, 50, 100)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	rows, total, err := a.store.ListActivity(r.Context(), limit, (page-1)*limit)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	out := make([]activityJSON, 0, len(rows))
	for _, x := range rows {
		out = append(out, activityJSON{
			ID: x.ID, At: fmtTime(x.At), UserID: x.UserID, Username: x.Username, Action: x.Action,
			ItemID: x.ItemID, ItemTitle: x.ItemTitle, Detail: x.Detail, Size: x.Size,
		})
	}
	writeJSON(w, http.StatusOK, pageJSON{Items: out, Total: total, Page: page, Limit: limit})
}

func (a *App) handleRefreshItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	it, err := a.store.GetItem(r.Context(), id)
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	d, err := a.buildDetail(r, it)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.scans.refresh(id)
	writeJSON(w, http.StatusAccepted, d)
}
