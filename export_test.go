package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeka/zip"
)

func testApp(t *testing.T, allowed string) *App {
	t.Helper()
	cfg := Config{}
	cfg.BackupAllIPs = mustCIDRs(t, allowed)
	return newApp(cfg, nil) // no database: only paths that return before any query
}

func post(a *App, h http.HandlerFunc, path, remote, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.RemoteAddr = remote
	r.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestBackupAllDisabledAndDenied(t *testing.T) {
	const body = `{"masterKey":"x","backupPassword":"yyyyyyyyyyyyyyyy"}`
	disabled := testApp(t, "")
	w := post(disabled, disabled.handleBackupAll, "/api/backup-all", "10.0.0.5:1", body, nil)
	if w.Code != 404 {
		t.Fatalf("disabled: %d", w.Code)
	}
	want := w.Body.String()

	a := testApp(t, "10.0.0.0/24,192.168.1.7")
	for name, c := range map[string]struct {
		remote string
		hdr    map[string]string
	}{
		"outside":                       {"203.0.113.9:1", nil},
		"spoofed xff":                   {"203.0.113.9:1", map[string]string{"X-Forwarded-For": "10.0.0.5"}},
		"allowed peer with xff":         {"10.0.0.5:1", map[string]string{"X-Forwarded-For": "10.0.0.5"}},
		"allowed peer with x-fwd-proto": {"10.0.0.5:1", map[string]string{"X-Forwarded-Proto": "https"}},
		"allowed peer with x-fwd-host":  {"10.0.0.5:1", map[string]string{"X-Forwarded-Host": "passman.example"}},
		"allowed peer with x-real-ip":   {"10.0.0.5:1", map[string]string{"X-Real-Ip": "10.0.0.5"}},
		"allowed peer with forwarded":   {"10.0.0.5:1", map[string]string{"Forwarded": "for=10.0.0.5"}},
		"ipv6 outside":                  {"[2001:db8::1]:1", nil},
		"unparsable":                    {"garbage", nil},
	} {
		w := post(a, a.handleBackupAll, "/api/backup-all", c.remote, body, c.hdr)
		if w.Code != 404 || w.Body.String() != want {
			t.Errorf("%s: %d %q", name, w.Code, w.Body.String())
		}
	}
	// GET from an allowed IP is also a plain 404.
	r := httptest.NewRequest(http.MethodGet, "/api/backup-all", nil)
	r.RemoteAddr = "10.0.0.5:1"
	rec := httptest.NewRecorder()
	a.handleBackupAll(rec, r)
	if rec.Code != 404 {
		t.Errorf("GET: %d", rec.Code)
	}
}

func TestBackupAllAllowedValidationAndThrottle(t *testing.T) {
	a := testApp(t, "10.0.0.0/24")
	for _, body := range []string{`{}`, `{"masterKey":"k"}`, `{"backupPassword":"pppppppppppppppp"}`, `{"masterKey":"  ","backupPassword":"pppppppppppppppp"}`} {
		if w := post(a, a.handleBackupAll, "/api/backup-all", "10.0.0.5:1", body, nil); w.Code != 400 {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	// IPv6 and v4-mapped peers are matched against the allow-list too.
	c := testApp(t, "fd00::/8,10.0.0.0/24")
	if w := post(c, c.handleBackupAll, "/api/backup-all", "[fd00::5]:1", `{}`, nil); w.Code != 400 {
		t.Errorf("ipv6 allowed: %d", w.Code)
	}
	if w := post(c, c.handleBackupAll, "/api/backup-all", "[::ffff:10.0.0.5]:1", `{}`, nil); w.Code != 400 {
		t.Errorf("v4-mapped allowed: %d", w.Code)
	}
	for i := 0; i < 5; i++ {
		a.masterAll.fail("*")
	}
	if w := post(a, a.handleBackupAll, "/api/backup-all", "10.0.0.5:1", `{"masterKey":"k","backupPassword":"pppppppppppppppp"}`, nil); w.Code != 429 {
		t.Errorf("global throttle: %d", w.Code)
	}
	b := testApp(t, "10.0.0.0/24")
	for i := 0; i < 5; i++ {
		b.masterIP.fail("10.0.0.5")
	}
	if w := post(b, b.handleBackupAll, "/api/backup-all", "10.0.0.5:1", `{"masterKey":"k","backupPassword":"pppppppppppppppp"}`, nil); w.Code != 429 {
		t.Errorf("ip throttle: %d", w.Code)
	}
	if w := post(b, b.handleBackupAll, "/api/backup-all", "10.0.0.6:1", `{}`, nil); w.Code != 400 {
		t.Errorf("other ip not throttled: %d", w.Code)
	}
}

func TestExportValidationAndThrottle(t *testing.T) {
	a := testApp(t, "")
	for _, body := range []string{`{}`, `{"username":"alice"}`, `{"password":"pw"}`, `{"username":"  ","password":"pw"}`} {
		if w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", body, nil); w.Code != 400 {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", `not json`, nil); w.Code != 400 {
		t.Errorf("bad json: %d", w.Code)
	}
	if w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", `{}`, map[string]string{"Content-Type": "text/plain"}); w.Code != 415 {
		t.Errorf("content type: %d", w.Code)
	}
	for i := 0; i < maxFails; i++ {
		a.limiter.fail("alice")
	}
	if w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", `{"username":"Alice","password":"pw"}`, nil); w.Code != 429 {
		t.Errorf("username throttle: %d", w.Code)
	}
	for i := 0; i < 20; i++ {
		a.exportIP.fail("198.51.100.2")
	}
	if w := post(a, a.handleExport, "/api/export", "198.51.100.2:1", `{"username":"bob","password":"pw"}`, nil); w.Code != 429 {
		t.Errorf("ip throttle: %d", w.Code)
	}
}

func TestSameOriginExemptionIsExact(t *testing.T) {
	h := sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	do := func(method, path string) int {
		r := httptest.NewRequest(method, path, nil)
		r.Host = "passman.example"
		r.Header.Set("Origin", "chrome-extension://abcdef")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for _, p := range []string{"/api/export", "/api/backup-all"} {
		if c := do("POST", p); c != 204 {
			t.Errorf("%s should be exempt, got %d", p, c)
		}
	}
	for _, p := range []string{"/api/login", "/api/backup", "/api/passwords", "/api/export/", "/api/export/x", "/api/exports"} {
		if c := do("POST", p); c != 403 {
			t.Errorf("%s must stay protected, got %d", p, c)
		}
	}
}

func TestLimiterWindow(t *testing.T) {
	l := newLimiter(2, 50*time.Millisecond)
	l.fail("k")
	l.fail("k")
	if !l.blocked("k") {
		t.Fatal("should block")
	}
	time.Sleep(70 * time.Millisecond)
	if l.blocked("k") {
		t.Fatal("window should have expired")
	}
}

func TestZipBackupsEncryptedAndNamed(t *testing.T) {
	raw, err := zipBackups("pw", map[string][]byte{"b.json": []byte(`{"b":1}`), "a.json": []byte(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 || zr.File[0].Name != "a.json" || zr.File[1].Name != "b.json" {
		t.Fatalf("files: %v", zr.File)
	}
	for _, f := range zr.File {
		if !f.IsEncrypted() {
			t.Errorf("%s not encrypted", f.Name)
		}
	}
	if bytes.Contains(raw, []byte(`"a":1`)) {
		t.Error("plaintext leaked into zip")
	}
}

func TestExportShortBackupPassword(t *testing.T) {
	a := testApp(t, "")
	w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", `{"username":"alice","password":"pw","backupPassword":"short"}`, nil)
	if w.Code != 400 {
		t.Fatalf("short backup password: %d", w.Code)
	}
}

func TestExemptPathsRequireJSONContentType(t *testing.T) {
	a := testApp(t, "10.0.0.0/24")
	h := a.routes()
	for _, p := range []string{"/api/export", "/api/backup-all"} {
		r := httptest.NewRequest(http.MethodPost, p, strings.NewReader(`{}`))
		r.RemoteAddr = "10.0.0.5:1"
		r.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 415 {
			t.Errorf("%s with text/plain: %d", p, w.Code)
		}
	}
}

func TestExemptPathsRefuseCrossSiteWebOrigins(t *testing.T) {
	h := sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	try := func(origin, site string) int {
		r := httptest.NewRequest("POST", "/api/export", nil)
		r.Host = "passman.example"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := try("https://evil.example", "cross-site"); c != 403 {
		t.Errorf("web page cross-site: %d", c)
	}
	if c := try("", "cross-site"); c != 403 {
		t.Errorf("no origin cross-site: %d", c)
	}
	if c := try("chrome-extension://abc", "cross-site"); c != 204 {
		t.Errorf("extension: %d", c)
	}
	if c := try("", ""); c != 204 {
		t.Errorf("curl: %d", c)
	}
}

func TestLimiterTakeIsAtomicAndReleasable(t *testing.T) {
	l := newLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.take("k") {
			t.Fatalf("take %d refused", i)
		}
	}
	if l.take("k") {
		t.Fatal("4th take must be refused before any work is done")
	}
	l.release("k")
	if !l.take("k") {
		t.Fatal("release should free a slot")
	}
	// Concurrent takers can never exceed the limit.
	m := newLimiter(5, time.Minute)
	var wg sync.WaitGroup
	var got int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.take("x") {
				atomic.AddInt32(&got, 1)
			}
		}()
	}
	wg.Wait()
	if got != 5 {
		t.Fatalf("admitted %d, want 5", got)
	}
}

func TestSemaphore(t *testing.T) {
	s := make(sem, 2)
	first, second, third := s.tryAcquire(), s.tryAcquire(), s.tryAcquire()
	if !first || !second || third {
		t.Fatal("capacity 2 expected")
	}
	s.release()
	if !s.tryAcquire() {
		t.Fatal("slot should be free after release")
	}
}

func TestHandlersAnswerBusyWhenSemaphoreFull(t *testing.T) {
	a := testApp(t, "10.0.0.0/24")
	a.kdf <- struct{}{}
	a.kdf <- struct{}{}
	if w := post(a, a.handleExport, "/api/export", "198.51.100.1:1", `{"username":"alice","password":"pw"}`, nil); w.Code != 429 || !strings.Contains(w.Body.String(), "Busy") {
		t.Errorf("export: %d %s", w.Code, w.Body.String())
	}
	if w := post(a, a.handleLogin, "/api/login", "198.51.100.1:1", `{"username":"alice","password":"pw"}`, nil); w.Code != 429 || !strings.Contains(w.Body.String(), "Busy") {
		t.Errorf("login: %d %s", w.Code, w.Body.String())
	}
	if a.limiter.fails["alice"] != nil {
		t.Error("a busy refusal must not count as an attempt")
	}
	a.kdfAll <- struct{}{}
	if w := post(a, a.handleBackupAll, "/api/backup-all", "10.0.0.5:1", `{"masterKey":"k","backupPassword":"pppppppppppppppp"}`, nil); w.Code != 429 || !strings.Contains(w.Body.String(), "Busy") {
		t.Errorf("backup-all: %d %s", w.Code, w.Body.String())
	}
}

func TestPlainZipAndNesting(t *testing.T) {
	inner, err := plainZip(map[string][]byte{"bob.json": []byte(`{"b":1}`), "alice.json": []byte(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := zipBackups("pw", map[string][]byte{"passman-all.zip": inner})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(outer, []byte("alice")) || bytes.Contains(outer, []byte("bob")) {
		t.Error("usernames visible in the outer zip")
	}
	zr, _ := zip.NewReader(bytes.NewReader(outer), int64(len(outer)))
	if len(zr.File) != 1 || zr.File[0].Name != "passman-all.zip" {
		t.Fatalf("outer entries: %v", zr.File)
	}
}
