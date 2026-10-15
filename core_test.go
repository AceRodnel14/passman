package main

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
	"unicode"
)

func TestPassphraseLengthAndShape(t *testing.T) {
	for length := minGenLength; length <= maxGenLength; length++ {
		for i := 0; i < 20; i++ {
			pw := generatePassphrase(length, false, false)
			if len(pw) != length {
				t.Fatalf("length %d: got %q (%d)", length, pw, len(pw))
			}
			if !unicode.IsUpper(rune(pw[0])) {
				t.Fatalf("not capitalised: %q", pw)
			}
			for _, r := range pw {
				if !unicode.IsLetter(r) {
					t.Fatalf("plain passphrase has non-letter: %q", pw)
				}
			}
		}
	}
}

func TestPassphraseSubstitutions(t *testing.T) {
	for i := 0; i < 500; i++ {
		num := generatePassphrase(40, true, false)
		both := generatePassphrase(40, true, true)
		sym := generatePassphrase(40, false, true)
		for _, r := range strings.ToLower(num) {
			if _, ok := digitSubs[r]; ok {
				t.Fatalf("numbers: letter %q left unswapped in %q", r, num)
			}
		}
		for _, r := range strings.ToLower(sym) {
			if _, ok := symbolSubs[r]; ok {
				t.Fatalf("symbols: letter %q left unswapped in %q", r, sym)
			}
		}
		for _, r := range strings.ToLower(both) {
			if _, ok := digitSubs[r]; ok {
				t.Fatalf("both: letter %q left unswapped in %q", r, both)
			}
			if _, ok := symbolSubs[r]; ok {
				t.Fatalf("both: letter %q left unswapped in %q", r, both)
			}
		}
		// symbol wins: digits for a/i/l/s must never appear when both are on
		if strings.ContainsAny(both, "4") {
			t.Fatalf("both: 'a' became a digit instead of @ in %q", both)
		}
		for _, r := range sym + num + both {
			if r > 126 {
				t.Fatalf("non-ASCII character in %q", sym+num+both)
			}
		}
	}
}

func TestPasswordRisk(t *testing.T) {
	cases := []struct {
		pw, user string
		want     int16
	}{
		{"short", "", riskWeak},
		{"password123", "", riskWeak},
		{"aaaaaaaaaaaa", "", riskWeak},
		{"abcdefgh", "", riskWeak},
		{"John2024!xyz", "john", riskUsername},
		{"MapleRiver", "john", riskNone},
		{"Tr0ub4dor&3x", "", riskNone},
	}
	for _, c := range cases {
		if got := passwordRisk(c.pw, c.user); got != c.want {
			t.Errorf("passwordRisk(%q, %q) = %d, want %d", c.pw, c.user, got, c.want)
		}
	}
}

func TestEscrowRoundTrip(t *testing.T) {
	priv, _ := ecdh.X25519().GenerateKey(rand.Reader)
	dek := randomBytes(keyLen)
	sealed, err := escrowSeal(priv.PublicKey(), dek, []byte("dek:alice"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := escrowOpen(priv, sealed, []byte("dek:alice"))
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := escrowOpen(priv, sealed, []byte("dek:bob")); err == nil {
		t.Fatal("escrow opened with the wrong AAD")
	}
}

func TestParseBackupRejectsJunk(t *testing.T) {
	for _, in := range []string{`{}`, `not json`, `{"version":2,"passwords":[],"folders":[]}`,
		`{"version":3,"passwords":[{"id":"nope"}],"folders":[]}`} {
		if _, err := parseBackup([]byte(in)); err == nil {
			t.Errorf("accepted %s", in)
		}
	}
	if _, err := parseBackup([]byte(`{"version":3,"encrypted":false,"passwords":[],"folders":[],"tags":[]}`)); err != nil {
		t.Errorf("rejected valid empty backup: %v", err)
	}
}

func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return iconImagePrefix + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestNormalizeIcon(t *testing.T) {
	ok := []*Icon{
		nil,
		{Mode: "auto", Color: "green"},
		{Mode: "letters", Text: "GH", Color: "blue"},
		{Mode: "letters", Text: "🔑"},
		{Mode: "logo", Image: pngDataURI(t, 64, 64), Color: "pink"},
		{Mode: "logo", Image: pngDataURI(t, 16, 16)},
	}
	for _, ic := range ok {
		if _, err := normalizeIcon(ic); err != nil {
			t.Errorf("rejected %+v: %v", ic, err)
		}
	}
	if got, _ := normalizeIcon(&Icon{Mode: "auto"}); got != nil {
		t.Errorf("plain auto icon should normalize to nil, got %+v", got)
	}
	bad := []*Icon{
		{Mode: "letters", Text: ""},
		{Mode: "letters", Text: "ABCDEFGHIJ"},
		{Mode: "letters", Text: "A", Color: "rainbow"},
		{Mode: "logo", Image: pngDataURI(t, 65, 64)},
		{Mode: "logo", Image: "data:image/svg+xml;base64,PHN2Zz4="},
		{Mode: "logo", Image: iconImagePrefix + "bm90IGEgcG5n"},
		{Mode: "sparkles"},
	}
	for _, ic := range bad {
		if _, err := normalizeIcon(ic); err == nil {
			t.Errorf("accepted %+v", ic)
		}
	}
}

func TestBackupIconOptional(t *testing.T) {
	nextcloud := `{"version":3,"encrypted":false,"tags":[],"folders":[],"passwords":[
		{"id":"eea52e84-0000-4000-8000-000000000001","label":"a","username":"","password":"p","notes":"","url":"",
		 "customFields":[],"folder":"00000000-0000-0000-0000-000000000000","edited":1,"favorite":false,"tags":[]}]}`
	b, err := parseBackup([]byte(nextcloud))
	if err != nil || b.Passwords[0].Icon != nil {
		t.Fatalf("Nextcloud export without icon: %v %+v", err, b)
	}
	withIcon := strings.Replace(nextcloud, `"tags":[]}]}`, `"tags":[],"icon":{"mode":"letters","text":"A","color":"blue"}}]}`, 1)
	b, err = parseBackup([]byte(withIcon))
	if err != nil || b.Passwords[0].Icon == nil || b.Passwords[0].Icon.Text != "A" {
		t.Fatalf("passman export with icon: %v %+v", err, b)
	}
}
