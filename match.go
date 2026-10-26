package main

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

var plainHostRe = regexp.MustCompile(`^[a-z0-9._-]+$`)

var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// parsedURL is a normalised http(s) URL; explicitScheme is false when the
// scheme was left out and defaulted.
type parsedURL struct {
	scheme         string
	explicitScheme bool
	host           string
	port           string // "" when not given
}

func defaultPort(scheme string) string {
	if scheme == "http" {
		return "80"
	}
	return "443"
}

func parseHTTPURL(raw string, allowNoScheme bool) (parsedURL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedURL{}, false
	}
	explicit := schemeRe.MatchString(raw)
	if !explicit {
		if !allowNoScheme {
			return parsedURL{}, false
		}
		raw = "//" + strings.TrimPrefix(raw, "//")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return parsedURL{}, false
	}
	p := parsedURL{explicitScheme: explicit, port: u.Port()}
	if explicit {
		p.scheme = strings.ToLower(u.Scheme)
		if p.scheme != "http" && p.scheme != "https" {
			return parsedURL{}, false
		}
	}
	p.host = strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if p.host == "" {
		return parsedURL{}, false
	}
	if !isIPHost(p.host) {
		ascii, err := idna.Lookup.ToASCII(p.host)
		if err != nil || ascii == "" {
			// Underscores etc. fail IDNA but are fine in intranet names.
			if !plainHostRe.MatchString(p.host) {
				return parsedURL{}, false
			}
			ascii = p.host
		}
		p.host = ascii
	}
	if p.port != "" {
		n, err := strconv.Atoi(p.port)
		if err != nil || n < 1 || n > 65535 {
			return parsedURL{}, false
		}
		p.port = strconv.Itoa(n)
		if explicit && p.port == defaultPort(p.scheme) {
			p.port = ""
		}
	}
	return p, true
}

func isIPHost(h string) bool {
	_, err := netip.ParseAddr(h)
	return err == nil
}

// pattern is the normalised, secret-free form of a stored entry URL. The
// browser extension caches these (GET /api/sites) and applies the same rules
// locally, so the server never has to learn which pages the user visits.
type pattern struct {
	Schemes    []string `json:"schemes"`    // page scheme must be one of these
	Host       string   `json:"host"`       // ASCII, lower-case, no trailing dot
	Port       string   `json:"port"`       // "" = default port of the page scheme
	Subdomains bool     `json:"subdomains"` // page host may be a subdomain of Host
}

// parsePattern turns a stored URL into a pattern; false if it is unusable.
func parsePattern(stored string) (pattern, bool) {
	st, ok := parseHTTPURL(stored, true)
	if !ok {
		return pattern{}, false
	}
	// IP literals and numeric last labels are never matched: sites are
	// identified by name only.
	if isIPHost(st.host) || numericLastLabel(st.host) {
		return pattern{}, false
	}
	exactOnly := !strings.Contains(st.host, ".")
	p := pattern{Host: st.host, Port: st.port}
	switch {
	case st.explicitScheme:
		p.Schemes = []string{st.scheme}
	case exactOnly:
		// Scheme-less single-label hosts (localhost) may use plain http.
		p.Schemes = []string{"http", "https"}
	default:
		p.Schemes = []string{"https"}
	}
	p.Subdomains = !exactOnly
	if p.Subdomains {
		// Never let a public suffix (co.uk, github.io) match its subdomains.
		if suffix, _ := publicsuffix.PublicSuffix(st.host); suffix == st.host {
			p.Subdomains = false
		}
	}
	return p, true
}

// numericLastLabel is true for hosts like "3.4": not IPs for netip, but a
// browser never treats an all-digit last label as a domain name.
func numericLastLabel(h string) bool {
	last := h[strings.LastIndex(h, ".")+1:]
	return last != "" && strings.Trim(last, "0123456789") == ""
}

// matches reports whether the pattern applies to page (a full URL or origin).
func (p pattern) matches(page string) bool {
	pg, ok := parseHTTPURL(page, false)
	if !ok {
		return false
	}
	return p.matchesParsed(pg)
}

func (p pattern) matchesParsed(pg parsedURL) bool {
	found := false
	for _, sc := range p.Schemes {
		if sc == pg.scheme {
			found = true
		}
	}
	if !found {
		return false
	}
	pagePort := pg.port
	if pagePort == "" {
		pagePort = defaultPort(pg.scheme)
	}
	want := p.Port
	if want == "" {
		want = defaultPort(pg.scheme)
	}
	if pagePort != want {
		return false
	}
	if pg.host == p.Host {
		return true
	}
	return p.Subdomains && !isIPHost(pg.host) && strings.HasSuffix(pg.host, "."+p.Host)
}

// urlMatches reports whether a stored entry URL should be offered on page.
func urlMatches(stored, page string) bool {
	p, ok := parsePattern(stored)
	return ok && p.matches(page)
}

type siteItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	pattern
}

// handleSites returns one item per saved entry with a usable URL (id, label
// and URL pattern only; no username or secrets) so the extension can keep the
// list on disk and match pages locally.
func (a *App) handleSites(w http.ResponseWriter, r *http.Request, s *Session) {
	v := s.vault()
	rows, err := a.db.Query(r.Context(), passwordSelect, v.userID)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	sites := []siteItem{}
	for rows.Next() {
		d, err := v.scanPassword(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		if p, ok := parsePattern(d.URL); ok {
			sites = append(sites, siteItem{ID: d.ID, Label: d.Label, pattern: p})
		}
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":           s.Username,
		"sessionExpires": s.Expires.UTC().Format(time.RFC3339),
		"sites":          sites,
	})
}

// handleFill returns the credentials of one entry, but only if its stored URL
// matches the given page origin. A mismatch is indistinguishable from a
// missing id.
func (a *App) handleFill(w http.ResponseWriter, r *http.Request, s *Session) {
	if _, ok := parseHTTPURL(r.URL.Query().Get("url"), false); !ok {
		writeError(w, http.StatusBadRequest, "A valid http(s) url is required.")
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	v := s.vault()
	d, err := v.scanPassword(a.db.QueryRow(r.Context(), passwordSelect+` AND p.id = $2::uuid`, v.userID, id))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !urlMatches(d.URL, r.URL.Query().Get("url"))) {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": d.Username, "password": d.Password})
}
