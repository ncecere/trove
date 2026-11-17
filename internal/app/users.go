package app

import (
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"trove/internal/store"
)

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, toUserJSON(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// parseLibraryIDs interprets a raw JSON libraryIds value: null => nil (all),
// array => the ids (deduplicated, must exist).
func (a *App) parseLibraryIDs(w http.ResponseWriter, r *http.Request, raw json.RawMessage) ([]int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		badRequest(w, "libraryIds must be null or an array of library ids")
		return nil, false
	}
	seen := map[int64]bool{}
	out := []int64{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	ok, err := a.store.LibrariesExist(r.Context(), out)
	if err != nil {
		a.internalError(w, r, err)
		return nil, false
	}
	if !ok {
		badRequest(w, "libraryIds contains an unknown library")
		return nil, false
	}
	return out, true
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username    string          `json:"username"`
		Password    string          `json:"password"`
		Role        string          `json:"role"`
		CanDownload *bool           `json:"canDownload"`
		LibraryIDs  json.RawMessage `json:"libraryIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.Username = trimmed(body.Username)
	if msg := validateUsername(body.Username); msg != "" {
		badRequest(w, msg)
		return
	}
	if msg := validatePassword(body.Password); msg != "" {
		badRequest(w, msg)
		return
	}
	if body.Role == "" {
		body.Role = "user"
	}
	if body.Role != "admin" && body.Role != "user" {
		badRequest(w, `role must be "admin" or "user"`)
		return
	}
	ids, ok := a.parseLibraryIDs(w, r, body.LibraryIDs)
	if !ok {
		return
	}
	canDL := true
	if body.CanDownload != nil {
		canDL = *body.CanDownload
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcryptCost)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	u, err := a.store.CreateUser(r.Context(), store.NewUser{
		Username: body.Username, PasswordHash: string(hash), Role: body.Role, CanDownload: canDL, LibraryIDs: ids,
	}, false)
	if errors.Is(err, store.ErrConflict) {
		conflict(w, "Username is already taken")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": toUserJSON(u)})
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	var body struct {
		Username    *string         `json:"username"`
		Password    *string         `json:"password"`
		Role        *string         `json:"role"`
		CanDownload *bool           `json:"canDownload"`
		LibraryIDs  json.RawMessage `json:"libraryIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var up store.UserUpdate
	if body.Username != nil {
		un := trimmed(*body.Username)
		if msg := validateUsername(un); msg != "" {
			badRequest(w, msg)
			return
		}
		up.Username = &un
	}
	if body.Password != nil {
		if msg := validatePassword(*body.Password); msg != "" {
			badRequest(w, msg)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*body.Password), bcryptCost)
		if err != nil {
			a.internalError(w, r, err)
			return
		}
		hs := string(hash)
		up.PasswordHash = &hs
	}
	if body.Role != nil {
		if *body.Role != "admin" && *body.Role != "user" {
			badRequest(w, `role must be "admin" or "user"`)
			return
		}
		up.Role = body.Role
	}
	up.CanDownload = body.CanDownload
	if len(body.LibraryIDs) > 0 {
		ids, ok := a.parseLibraryIDs(w, r, body.LibraryIDs)
		if !ok {
			return
		}
		up.LibraryIDsSet, up.LibraryIDs = true, ids
	}
	u, err := a.store.UpdateUser(r.Context(), id, up)
	switch {
	case errors.Is(err, store.ErrLastAdmin):
		conflict(w, "Cannot demote the last remaining admin")
		return
	case errors.Is(err, store.ErrConflict):
		conflict(w, "Username is already taken")
		return
	case err != nil:
		a.storeErr(w, r, err)
		return
	}
	if up.PasswordHash != nil {
		keep := ""
		if id == userFrom(r).ID {
			keep = sessionHashFrom(r)
		}
		if err := a.store.DeleteUserSessions(r.Context(), id, keep); err != nil {
			a.internalError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserJSON(u)})
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	if id == userFrom(r).ID {
		conflict(w, "You cannot delete your own account")
		return
	}
	err := a.store.DeleteUser(r.Context(), id)
	if errors.Is(err, store.ErrLastAdmin) {
		conflict(w, "Cannot delete the last remaining admin")
		return
	}
	if err != nil {
		a.storeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
