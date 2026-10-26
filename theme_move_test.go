package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func intp(i int) *int { return &i }

func TestThemeValidate(t *testing.T) {
	tests := []struct {
		name string
		in   themeInput
		ok   bool
	}{
		{"auto nil hue", themeInput{"auto", nil}, true},
		{"light hue 0", themeInput{"light", intp(0)}, true},
		{"dark hue 359", themeInput{"dark", intp(359)}, true},
		{"hue -1", themeInput{"dark", intp(-1)}, false},
		{"hue 360", themeInput{"dark", intp(360)}, false},
		{"empty mode", themeInput{"", nil}, false},
		{"bad mode", themeInput{"sepia", nil}, false},
		{"mode case", themeInput{"Dark", nil}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th, ok := tc.in.validate()
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && (th.Mode != tc.in.Mode || (tc.in.Hue == nil) != (th.Hue == nil) || (th.Hue != nil && int(*th.Hue) != *tc.in.Hue)) {
				t.Fatalf("got %+v from %+v", th, tc.in)
			}
		})
	}
}

// moveHandler wires the handler through a mux (for PathValue) with a fake
// session, so no database is touched on the paths under test.
func moveHandler(a *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/passwords/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		a.handleMovePassword(w, r, &Session{UserID: 1, Username: "Alice"})
	})
	return mux
}

func doMove(h http.Handler, id, ctype, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/passwords/"+id+"/move", strings.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMovePasswordEarlyRejections(t *testing.T) {
	const goodID = "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name         string
		id, ctype    string
		body         string
		busy, locked bool
		want         int
		msg          string
	}{
		{"bad id", "nope", "application/json", `{"password":"x"}`, false, false, 404, "Not found."},
		{"no content type", goodID, "", `{"password":"x"}`, false, false, 415, "Expected JSON."},
		{"bad json", goodID, "application/json", `{`, false, false, 400, "Invalid request."},
		{"empty password", goodID, "application/json", `{"folder":""}`, false, false, 400, "Password is required."},
		{"kdf busy", goodID, "application/json", `{"password":"x"}`, true, false, 429, "Busy, try again."},
		{"rate limited", goodID, "application/json", `{"password":"x"}`, false, true, 429, "Too many failed attempts. Try again in a few minutes."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{limiter: newLimiter(maxFails, failWindow), kdf: make(sem, 1)}
			if tc.busy {
				a.kdf.tryAcquire()
			}
			if tc.locked {
				for i := 0; i < maxFails; i++ {
					a.limiter.fail("alice")
				}
			}
			rec := doMove(moveHandler(a), tc.id, tc.ctype, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tc.msg) {
				t.Fatalf("body %q lacks %q", rec.Body, tc.msg)
			}
		})
	}
}

func TestSetThemeEarlyRejections(t *testing.T) {
	a := &App{}
	h := func(w http.ResponseWriter, r *http.Request) { a.handleSetTheme(w, r, &Session{UserID: 1}) }
	tests := []struct {
		name, ctype, body string
		want              int
	}{
		{"415", "text/plain", `{}`, 415},
		{"malformed", "application/json", `{`, 400},
		{"bad mode", "application/json", `{"mode":"x","hue":null}`, 400},
		{"hue 360", "application/json", `{"mode":"dark","hue":360}`, 400},
		{"hue -1", "application/json", `{"mode":"dark","hue":-1}`, 400},
		{"hue string", "application/json", `{"mode":"dark","hue":"5"}`, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/account/theme", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.ctype)
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestFolderChanged(t *testing.T) {
	const f1 = "11111111-1111-1111-1111-111111111111"
	const f2 = "22222222-2222-2222-2222-222222222222"
	up := "11111111-1111-1111-1111-111111111111"
	s := func(v string) *string { return &v }
	tests := []struct {
		name    string
		current *string
		req     string
		want    bool
	}{
		{"root to root empty", nil, "", false},
		{"root to root uuid", nil, rootFolderID, false},
		{"root to folder", nil, f1, true},
		{"folder to root empty", s(f1), "", true},
		{"folder to root uuid", s(f1), rootFolderID, true},
		{"same folder", s(f1), f1, false},
		{"same folder other case", s(up), strings.ToUpper(f1), false},
		{"other folder", s(f1), f2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := folderChanged(tc.current, tc.req); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReauthRejections(t *testing.T) {
	tests := []struct {
		name         string
		pw           string
		busy, locked bool
		want         int
	}{
		{"empty", "", false, false, 400},
		{"busy", "x", true, false, 429},
		{"limited", "x", false, true, 429},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{limiter: newLimiter(maxFails, failWindow), kdf: make(sem, 1)}
			if tc.busy {
				a.kdf.tryAcquire()
			}
			if tc.locked {
				for i := 0; i < maxFails; i++ {
					a.limiter.fail("alice")
				}
			}
			rec := httptest.NewRecorder()
			ok := a.reauth(rec, httptest.NewRequest(http.MethodPut, "/", nil), &Session{Username: "Alice"}, tc.pw)
			if ok || rec.Code != tc.want {
				t.Fatalf("ok=%v status=%d, want false/%d", ok, rec.Code, tc.want)
			}
		})
	}
}

func TestSavePasswordEarlyRejections(t *testing.T) {
	a := &App{}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/passwords/{id}", func(w http.ResponseWriter, r *http.Request) {
		a.handleSavePassword(w, r, &Session{UserID: 1, Username: "alice"})
	})
	tests := []struct {
		name, id, ctype, body string
		want                  int
	}{
		{"415", "11111111-1111-1111-1111-111111111111", "text/plain", `{}`, 415},
		{"no label", "11111111-1111-1111-1111-111111111111", "application/json", `{"password":"x"}`, 400},
		{"bad id", "nope", "application/json", `{"label":"a","password":"x"}`, 404},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/passwords/"+tc.id, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.ctype)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
