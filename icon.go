package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"
	"unicode/utf8"
)

// Icon customises the tile shown next to a password. A nil icon means
// "auto": the first letter of the name on a colour derived from the name.
type Icon struct {
	Mode  string `json:"mode"`            // "auto", "letters" or "logo"
	Color string `json:"color,omitempty"` // palette key; empty = derived from the name
	Text  string `json:"text,omitempty"`  // letters mode: 1–2 characters
	Image string `json:"image,omitempty"` // logo mode: data:image/png;base64,… at most 64×64
}

const (
	iconImagePrefix = "data:image/png;base64,"
	maxIconPNGBytes = 32 << 10
	maxIconSide     = 64
)

var iconColors = map[string]bool{"blue": true, "green": true, "purple": true, "orange": true, "pink": true, "yellow": true}

// normalizeIcon validates an icon and returns nil when it carries nothing
// beyond the default look.
func normalizeIcon(ic *Icon) (*Icon, error) {
	if ic == nil {
		return nil, nil
	}
	if ic.Color != "" && !iconColors[ic.Color] {
		return nil, errors.New("Unknown icon colour.")
	}
	switch ic.Mode {
	case "", "auto":
		if ic.Color == "" {
			return nil, nil
		}
		return &Icon{Mode: "auto", Color: ic.Color}, nil
	case "letters":
		text := strings.TrimSpace(ic.Text)
		// The browser limits this to 2 visible characters; emoji can take several code points.
		if text == "" || len(text) > 32 || utf8.RuneCountInString(text) > 8 {
			return nil, errors.New("Icon letters must be 1 or 2 characters.")
		}
		return &Icon{Mode: "letters", Color: ic.Color, Text: text}, nil
	case "logo":
		if err := checkIconImage(ic.Image); err != nil {
			return nil, err
		}
		return &Icon{Mode: "logo", Color: ic.Color, Image: ic.Image}, nil
	}
	return nil, errors.New("Unknown icon type.")
}

// checkIconImage accepts only a small, valid PNG; the browser does the resizing.
func checkIconImage(s string) error {
	b64, ok := strings.CutPrefix(s, iconImagePrefix)
	if !ok {
		return errors.New("Icon image must be a PNG.")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) > maxIconPNGBytes {
		return errors.New("Icon image is invalid or too large.")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return errors.New("Icon image is not a valid PNG.")
	}
	if b := img.Bounds(); b.Dx() < 1 || b.Dy() < 1 || b.Dx() > maxIconSide || b.Dy() > maxIconSide {
		return errors.New("Icon image must be at most 64×64 pixels.")
	}
	return nil
}
