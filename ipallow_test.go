package main

import (
	"net/http"
	"net/netip"
	"testing"
)

func mustCIDRs(t *testing.T, s string) []netip.Prefix {
	t.Helper()
	p, err := parseCIDRs(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPeerIP(t *testing.T) {
	cases := []struct{ remote, xff, want string }{
		{"203.0.113.9:5555", "", "203.0.113.9"},
		{"203.0.113.9:5555", "10.0.0.1", "203.0.113.9"}, // headers never change the IP
		{"[2001:db8::9]:1", "", "2001:db8::9"},
		{"[::ffff:10.0.0.2]:1", "", "10.0.0.2"},
		{"10.0.0.2", "", "10.0.0.2"}, // no port
		{"nonsense", "", ""},
		{"", "", ""},
		{"10.0.0.2:notaport", "", ""},
	}
	for _, c := range cases {
		r := &http.Request{RemoteAddr: c.remote, Header: http.Header{}}
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		got := peerIP(r)
		if c.want == "" {
			if got.IsValid() {
				t.Errorf("%q: got %v, want none", c.remote, got)
			}
		} else if got.String() != c.want {
			t.Errorf("%q: got %v want %s", c.remote, got, c.want)
		}
	}
}

func TestViaProxy(t *testing.T) {
	for _, h := range forwardingHeaders {
		r := &http.Request{Header: http.Header{}}
		r.Header.Set(h, "x")
		if !viaProxy(r) {
			t.Errorf("%s not detected", h)
		}
	}
	if viaProxy(&http.Request{Header: http.Header{"User-Agent": {"curl"}}}) {
		t.Error("false positive")
	}
}

func TestParseCIDRs(t *testing.T) {
	ok := map[string][]string{
		"":                        nil,
		" , ":                     nil,
		"10.0.0.0/8":              {"10.0.0.0/8"},
		"192.168.1.5":             {"192.168.1.5/32"},
		"2001:db8::1":             {"2001:db8::1/128"},
		"10.1.2.3/8":              {"10.0.0.0/8"},
		" 10.0.0.0/8 , fd00::/8 ": {"10.0.0.0/8", "fd00::/8"},
		"::ffff:10.0.0.0/104":     {"10.0.0.0/8"},
	}
	for in, want := range ok {
		got, err := parseCIDRs(in)
		if err != nil || len(got) != len(want) {
			t.Fatalf("%q: %v %v", in, got, err)
		}
		for i := range want {
			if got[i].String() != want[i] {
				t.Errorf("%q: got %v want %v", in, got, want)
			}
		}
	}
	for _, bad := range []string{"nope", "10.0.0.0/33", "10.0.0.0/x", "1.2.3", "10.0.0.0/8,zzz"} {
		if _, err := parseCIDRs(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}
