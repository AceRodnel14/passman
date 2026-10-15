package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Admins see account names only. Vault contents stay encrypted under each
// user's own key and are never exposed through the admin API.

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request, s *Session) {
	type userView struct {
		ID                int64     `json:"id"`
		Username          string    `json:"username"`
		IsAdmin           bool      `json:"isAdmin"`
		CreatedAt         time.Time `json:"createdAt"`
		PasswordChangedAt time.Time `json:"passwordChangedAt"`
	}
	rows, err := a.db.Query(r.Context(),
		`SELECT id, username, is_admin, created_at, password_changed_at FROM users ORDER BY is_admin DESC, lower(username)`)
	if err != nil {
		serverError(w, err)
		return
	}
	users := []userView{}
	for rows.Next() {
		var u userView
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt, &u.PasswordChangedAt); err != nil {
			serverError(w, err)
			return
		}
		users = append(users, u)
	}
	if rows.Err() != nil {
		serverError(w, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *App) handleAdminResetPassword(w http.ResponseWriter, r *http.Request, s *Session) {
	var req struct{ MasterKey, Password, Confirm string }
	if !readJSON(w, r, &req) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	u, err := userByID(r.Context(), a.db, id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "User not found.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	if err := validateNewPassword(req.Password, req.Confirm); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var keep []byte
	if u.ID == s.UserID {
		keep = s.token
	}
	err = resetPasswordWithMasterKey(r.Context(), a.db, strings.TrimSpace(req.MasterKey), u.Username, req.Password, keep)
	if errors.Is(err, ErrBadMasterKey) {
		writeError(w, http.StatusBadRequest, "That master key is not correct.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
