package declarative

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// dateMacroRE matches {date:now}, {date:-Nd}, {date:+Nd}, {date:-Nh}, {date:+Nh}.
var dateMacroRE = regexp.MustCompile(`\{date:(now|[+-]\d+[dh])\}`)

// expandDateMacros replaces {date:...} tokens in a URL with a UTC date string
// (YYYY-MM-DD) computed relative to `now`, so a static source URL can express a
// rolling window. Time-windowed APIs (e.g. Global Fishing Watch events, which
// require absolute start-date/end-date) need this because the declarative URL is
// otherwise fixed. Expansion happens at FETCH time (not load time) so the window
// rolls forward on every poll.
//
// Supported tokens:
//
//	{date:now}   -> today (UTC)
//	{date:-7d}   -> 7 days ago
//	{date:+1d}   -> 1 day ahead
//	{date:-6h}   -> 6 hours ago (still rendered as a date)
//
// Unknown/malformed tokens are left untouched.
func expandDateMacros(rawURL string, now time.Time) string {
	if !strings.Contains(rawURL, "{date:") {
		return rawURL
	}
	return dateMacroRE.ReplaceAllStringFunc(rawURL, func(match string) string {
		spec := match[len("{date:") : len(match)-1] // strip "{date:" and "}"
		t := now
		if spec != "now" {
			n, err := strconv.Atoi(spec[1 : len(spec)-1]) // digits between sign and unit
			if err != nil {
				return match // leave malformed tokens untouched
			}
			d := time.Duration(n) * 24 * time.Hour
			if spec[len(spec)-1] == 'h' {
				d = time.Duration(n) * time.Hour
			}
			if spec[0] == '-' {
				d = -d
			}
			t = now.Add(d)
		}
		return t.UTC().Format("2006-01-02")
	})
}
