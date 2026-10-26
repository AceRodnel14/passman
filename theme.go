package main

import (
	"net/http"
)

const (
	themeAuto  = "auto"
	themeLight = "light"
	themeDark  = "dark"
)

// Theme is a user's appearance preference. A nil Hue is the original blue.
type Theme struct {
	Mode string `json:"mode"`
	Hue  *int16 `json:"hue"`
}

type themeInput struct {
	Mode string `json:"mode"`
	Hue  *int   `json:"hue"`
}

// validate checks the mode enum and the hue range (0..359 or nil).
func (in themeInput) validate() (Theme, bool) {
	switch in.Mode {
	case themeAuto, themeLight, themeDark:
	default:
		return Theme{}, false
	}
	t := Theme{Mode: in.Mode}
	if in.Hue != nil {
		if *in.Hue < 0 || *in.Hue > 359 {
			return Theme{}, false
		}
		h := int16(*in.Hue)
		t.Hue = &h
	}
	return t, true
}

func (a *App) handleSetTheme(w http.ResponseWriter, r *http.Request, s *Session) {
	var in themeInput
	if !readJSON(w, r, &in) {
		return
	}
	t, ok := in.validate()
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid theme.")
		return
	}
	if _, err := a.db.Exec(r.Context(), `UPDATE users SET theme_mode = $2, theme_hue = $3 WHERE id = $1`, s.UserID, t.Mode, t.Hue); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
