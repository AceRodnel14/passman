package main

import (
	"encoding/json"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/idna"
)

type matchCase struct {
	name, stored, page string
	want               bool
}

var matchCases = []matchCase{
	{"exact", "https://example.com", "https://example.com/login?x=1#f", true},
	{"scheme-less", "example.com", "https://example.com/", true},
	{"scheme-less not on http", "example.com", "http://example.com/", false},
	{"scheme-less port on http", "example.com:8080", "http://example.com:8080/", false},
	{"explicit http entry on http", "http://example.com", "http://example.com/", true},
	{"scheme-less localhost on http", "localhost:3000", "http://localhost:3000/", true},
	{"scheme-less ip never matches", "127.0.0.1:8080", "http://127.0.0.1:8080/", false},
	{"scheme-less ip default port", "192.168.1.5", "http://192.168.1.5/", false},
	{"userinfo stored", "https://user:pw@example.com/login", "https://example.com/", true},
	{"userinfo scheme-less", "user@example.com", "https://example.com/", true},
	{"empty port", "example.com:", "https://example.com/", true},
	{"scheme-less with :// in query", "example.com/?r=http://x.test", "https://example.com/", true},
	{"scheme-less :// query not http", "example.com/?r=http://x.test", "http://example.com/", false},
	{"idn stored, punycode page", "bücher.de", "https://xn--bcher-kva.de/", true},
	{"idn page, punycode stored", "xn--bcher-kva.de", "https://bücher.de/", true},
	{"idn subdomain", "bücher.de", "https://shop.xn--bcher-kva.de/", true},
	{"idn uppercase", "BÜCHER.de", "https://bücher.de/", true},
	{"public suffix no subdomains", "co.uk", "https://example.co.uk/", false},
	{"public suffix github.io", "https://github.io", "https://user.github.io/", false},
	{"public suffix exact", "github.io", "https://github.io/", true},
	{"private domain under suffix", "user.github.io", "https://a.user.github.io/", true},
	{"scheme-less path", "example.com/login", "https://example.com/other", true},
	{"https entry not on http", "https://example.com", "http://example.com", false},
	{"http entry not on https", "http://example.com", "https://example.com", false},
	{"case and trailing dot", "EXAMPLE.com.", "https://Example.COM./", true},
	{"subdomain", "example.com", "https://login.example.com", true},
	{"deep subdomain", "example.com", "https://a.b.example.com", true},
	{"bad suffix", "example.com", "https://badexample.com", false},
	{"parent not matched", "login.example.com", "https://example.com", false},
	{"sibling", "a.example.com", "https://b.example.com", false},
	{"stored port match", "example.com:8443", "https://example.com:8443/", true},
	{"stored port mismatch", "example.com:8443", "https://example.com/", false},
	{"no stored port, page custom port", "example.com", "https://example.com:8443/", false},
	{"explicit default port", "https://example.com:443", "https://example.com", true},
	{"page explicit default port", "http://example.com", "http://example.com:80/", true},
	{"scheme-less port 80 on https", "example.com:80", "https://example.com/", false},
	{"ipv4 never matches", "192.168.1.5", "http://192.168.1.5/", false},
	{"ipv4 no suffix", "3.4", "http://1.2.3.4/", false},
	{"ip page, dotted stored", "3.4", "https://1.2.3.4/", false},
	{"ip page, https dotted stored", "https://3.4", "https://1.2.3.4/", false},
	{"numeric last label exact", "0.1", "https://0.1/", false},
	{"port leading zeros", "https://example.com:0443", "https://example.com/", true},
	{"port default explicit scheme", "https://example.com:443", "https://example.com:443/", true},
	{"port leading zeros custom", "example.com:08443", "https://example.com:8443/", true},
	{"port zero", "example.com:0", "https://example.com/", false},
	{"port too big", "example.com:70000", "https://example.com/", false},
	{"underscore host", "my_host.lan", "https://my_host.lan/", true},
	{"underscore subdomain", "my_host.lan", "https://a.my_host.lan/", true},
	{"underscore uppercase", "My_Host.lan", "https://my_host.lan/", true},
	{"underscore page, plain stored", "lan", "https://my_host.lan/", false},
	{"ipv4 other", "192.168.1.5", "http://192.168.1.6/", false},
	{"ipv6 never matches", "https://[::1]:8443", "https://[::1]:8443/x", false},
	{"ipv6 other", "https://[::1]", "https://[::2]/", false},
	{"localhost exact", "localhost:3000", "http://localhost:3000/", true},
	{"localhost no subdomain", "localhost", "http://app.localhost/", false},
	{"empty stored", "", "https://example.com", false},
	{"blank stored", "   ", "https://example.com", false},
	{"unparsable stored", "http://exa mple.com", "https://example.com", false},
	{"ftp stored", "ftp://example.com", "https://example.com", false},
	{"page ftp", "example.com", "ftp://example.com", false},
	{"page no scheme", "example.com", "example.com", false},
	{"page no host", "example.com", "https:///path", false},
	{"page empty", "example.com", "", false},
	{"page javascript", "example.com", "javascript:alert(1)", false},
}

func TestURLMatches(t *testing.T) {
	for _, c := range matchCases {
		t.Run(c.name, func(t *testing.T) {
			if got := urlMatches(c.stored, c.page); got != c.want {
				t.Fatalf("urlMatches(%q, %q) = %v, want %v", c.stored, c.page, got, c.want)
			}
		})
	}
}

// clientMatch is the algorithm documented in the README for the extension,
// written against the JSON fields only (no shared Go code with the server).
func clientMatch(site map[string]any, page string) bool {
	u, err := url.Parse(page)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if _, err := netip.ParseAddr(host); err != nil {
		if ascii, err := idna.Lookup.ToASCII(host); err == nil {
			host = ascii // browsers keep hosts like my_host.lan as they are
		}
	}
	ok := false
	for _, sc := range site["schemes"].([]any) {
		if sc == u.Scheme {
			ok = true
		}
	}
	if !ok {
		return false
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	want, _ := site["port"].(string)
	if want == "" {
		want = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	if port != want {
		return false
	}
	h := site["host"].(string)
	if host == h {
		return true
	}
	_, ipErr := netip.ParseAddr(host)
	return site["subdomains"] == true && ipErr != nil && strings.HasSuffix(host, "."+h)
}

func TestPatternClientAlgorithmAgrees(t *testing.T) {
	type out struct {
		Stored  string          `json:"stored"`
		Page    string          `json:"page"`
		Pattern json.RawMessage `json:"pattern"`
		Match   bool            `json:"match"`
	}
	var cases []out
	for _, c := range matchCases {
		var site map[string]any
		raw := json.RawMessage("null")
		if p, ok := parsePattern(c.stored); ok {
			b, _ := json.Marshal(p)
			raw = b
			if err := json.Unmarshal(b, &site); err != nil {
				t.Fatal(err)
			}
		}
		got := site != nil && clientMatch(site, c.page)
		if got != c.want {
			t.Errorf("%s: client algorithm = %v, want %v (pattern %s)", c.name, got, c.want, raw)
		}
		cases = append(cases, out{c.stored, c.page, raw, c.want})
	}
	if path := os.Getenv("MATCH_CASES_OUT"); path != "" {
		b, _ := json.MarshalIndent(cases, "", "  ")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSiteItemJSONShape(t *testing.T) {
	p, _ := parsePattern("instagram.com")
	b, _ := json.Marshal(siteItem{ID: "abc", Label: "Insta", pattern: p})
	if string(b) != `{"id":"abc","label":"Insta","schemes":["https"],"host":"instagram.com","port":"","subdomains":true}` {
		t.Fatalf("got %s", b)
	}
	for _, bad := range []string{"192.168.1.5", "[::1]", "https://[::1]:8443", "3.4", "example.1"} {
		if _, ok := parsePattern(bad); ok {
			t.Errorf("%q must be unusable", bad)
		}
	}
}

func TestPatternJSONShape(t *testing.T) {
	p, _ := parsePattern("localhost:3000")
	b, _ := json.Marshal(p)
	if string(b) != `{"schemes":["http","https"],"host":"localhost","port":"3000","subdomains":false}` {
		t.Fatalf("got %s", b)
	}
	if _, ok := parsePattern("  "); ok {
		t.Fatal("blank must be unusable")
	}
}

func TestVersionSet(t *testing.T) {
	if version == "" {
		t.Fatal("version must have a default")
	}
}
