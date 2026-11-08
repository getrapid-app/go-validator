package validator

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func registerStrings(r *Registry) {
	r.Register("alpha", func(p []string) (Rule, error) {
		return alphaRule("alpha", containsString(p, "ascii"), alphaLetters), nil
	}).
		Register("alpha_dash", func(p []string) (Rule, error) {
			return alphaRule("alpha_dash", containsString(p, "ascii"), alphaDash), nil
		}).
		Register("alpha_num", func(p []string) (Rule, error) {
			return alphaRule("alpha_num", containsString(p, "ascii"), alphaNum), nil
		}).
		Register("ascii", fixed(ASCII())).
		Register("lowercase", fixed(Lowercase())).
		Register("uppercase", fixed(Uppercase())).
		Register("starts_with", atLeast(1, "at least one prefix", func(p []string) (Rule, error) { return StartsWith(p...), nil })).
		Register("ends_with", atLeast(1, "at least one suffix", func(p []string) (Rule, error) { return EndsWith(p...), nil })).
		Register("doesnt_start_with", atLeast(1, "at least one prefix", func(p []string) (Rule, error) { return DoesntStartWith(p...), nil })).
		Register("doesnt_end_with", atLeast(1, "at least one suffix", func(p []string) (Rule, error) { return DoesntEndWith(p...), nil })).
		Register("regex", atLeast(1, "a pattern", func(p []string) (Rule, error) { return compileRegex("regex", p[0], true) })).
		Register("not_regex", atLeast(1, "a pattern", func(p []string) (Rule, error) { return compileRegex("not_regex", p[0], false) })).
		Register("email", func(p []string) (Rule, error) { return emailRule(p) }).
		Register("url", values(URL)).
		Register("active_url", fixed(ActiveURL())).
		Register("uuid", func(p []string) (Rule, error) {
			if len(p) == 0 {
				return UUID(), nil
			}
			n, err := strconv.Atoi(p[0])
			if err != nil || n < 1 || n > 8 {
				return nil, fmt.Errorf("needs a version from 1 to 8, got %q", p[0])
			}
			return UUIDVersion(n), nil
		}).
		Register("ulid", fixed(ULID())).
		Register("ip", fixed(IP())).
		Register("ipv4", fixed(IPv4())).
		Register("ipv6", fixed(IPv6())).
		Register("mac_address", fixed(MACAddress())).
		Register("hex_color", fixed(HexColor()))
}

// textOf returns a string or number value as text. Most string rules accept
// numbers too, as Laravel's do.
func textOf(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool, nil:
		return "", false
	}
	if _, ok := toFloat(v); ok {
		return toString(v), true
	}
	return "", false
}

func stringCheck(name string, params map[string]any, ok func(s string) bool) Rule {
	return check(name, params, func(f Field) bool {
		s, isText := textOf(f.Value)
		return isText && ok(s)
	})
}

// --- character classes -------------------------------------------------------

func alphaLetters(r rune) bool { return unicode.IsLetter(r) || unicode.Is(unicode.M, r) }
func alphaNum(r rune) bool     { return alphaLetters(r) || unicode.IsNumber(r) }
func alphaDash(r rune) bool    { return alphaNum(r) || r == '-' || r == '_' }

func alphaRule(name string, ascii bool, allowed func(rune) bool) Rule {
	return stringCheck(name, nil, func(s string) bool {
		if s == "" {
			return false
		}
		for _, r := range s {
			if ascii && r > unicode.MaxASCII {
				return false
			}
			if !allowed(r) {
				return false
			}
		}
		return true
	})
}

// Alpha passes strings made only of letters. With ascii, only a-z and A-Z.
func Alpha(ascii ...bool) Rule { return alphaRule("alpha", len(ascii) > 0 && ascii[0], alphaLetters) }

// AlphaDash passes letters, numbers, dashes and underscores.
func AlphaDash(ascii ...bool) Rule {
	return alphaRule("alpha_dash", len(ascii) > 0 && ascii[0], alphaDash)
}

// AlphaNum passes letters and numbers.
func AlphaNum(ascii ...bool) Rule {
	return alphaRule("alpha_num", len(ascii) > 0 && ascii[0], alphaNum)
}

// ASCII passes strings made only of 7-bit ASCII characters.
func ASCII() Rule {
	return stringCheck("ascii", nil, func(s string) bool {
		for i := 0; i < len(s); i++ {
			if s[i] > unicode.MaxASCII {
				return false
			}
		}
		return true
	})
}

// Lowercase passes strings with no uppercase letters.
func Lowercase() Rule {
	return stringCheck("lowercase", nil, func(s string) bool { return s == strings.ToLower(s) })
}

// Uppercase passes strings with no lowercase letters.
func Uppercase() Rule {
	return stringCheck("uppercase", nil, func(s string) bool { return s == strings.ToUpper(s) })
}

// --- prefixes and suffixes -----------------------------------------------------

func affixRule(name string, list []string, want bool, has func(s, x string) bool) Rule {
	return stringCheck(name, map[string]any{"values": list}, func(s string) bool {
		for _, x := range list {
			if has(s, x) {
				return want
			}
		}
		return !want
	})
}

// StartsWith passes strings that start with one of prefixes.
func StartsWith(prefixes ...string) Rule {
	return affixRule("starts_with", prefixes, true, strings.HasPrefix)
}

// EndsWith passes strings that end with one of suffixes.
func EndsWith(suffixes ...string) Rule {
	return affixRule("ends_with", suffixes, true, strings.HasSuffix)
}

// DoesntStartWith passes strings that start with none of prefixes.
func DoesntStartWith(prefixes ...string) Rule {
	return affixRule("doesnt_start_with", prefixes, false, strings.HasPrefix)
}

// DoesntEndWith passes strings that end with none of suffixes.
func DoesntEndWith(suffixes ...string) Rule {
	return affixRule("doesnt_end_with", suffixes, false, strings.HasSuffix)
}

// --- regular expressions -------------------------------------------------------

// Regex passes values matching pattern. pattern is either Go syntax
// ("^[a-z]+$") or PHP-style with delimiters and flags ("/^[a-z]+$/i"), so
// rules copied from Laravel keep working.
func Regex(pattern string) Rule { return mustRegex("regex", pattern, true) }

// NotRegex passes values that do not match pattern.
func NotRegex(pattern string) Rule { return mustRegex("not_regex", pattern, false) }

// RegexOf passes values matching an already compiled expression.
func RegexOf(re *regexp.Regexp) Rule { return regexRule("regex", re, true) }

func mustRegex(name, pattern string, want bool) Rule {
	r, err := compileRegex(name, pattern, want)
	if err != nil {
		panic("validator: " + err.Error())
	}
	return r
}

func compileRegex(name, pattern string, want bool) (Rule, error) {
	re, err := regexp.Compile(goPattern(pattern))
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return regexRule(name, re, want), nil
}

func regexRule(name string, re *regexp.Regexp, want bool) Rule {
	return stringCheck(name, nil, func(s string) bool { return re.MatchString(s) == want })
}

// goPattern converts a PHP-delimited pattern such as "/abc/i" to Go syntax.
// Anything else is returned unchanged.
func goPattern(p string) string {
	if len(p) < 2 {
		return p
	}
	d := p[0]
	if d != '/' && d != '#' && d != '~' && d != '%' && d != '!' && d != '@' && d != '|' {
		return p
	}
	end := strings.LastIndexByte(p, d)
	if end <= 0 {
		return p
	}
	body, flags := p[1:end], p[end+1:]
	var goFlags string
	for _, c := range flags {
		switch c {
		case 'i', 'm', 's', 'U':
			goFlags += string(c)
		case 'u', 'D', 'x':
			// u: Go is always UTF-8; D and x: closest behaviour is to ignore.
		default:
			return p
		}
	}
	if goFlags != "" {
		return "(?" + goFlags + ")" + body
	}
	return body
}

// --- email -----------------------------------------------------------------------

// EmailStyle selects how strictly Email checks addresses, like Laravel's
// email:rfc,dns.
type EmailStyle string

const (
	EmailRFC    EmailStyle = "rfc"    // RFC 5322 address (the default)
	EmailStrict EmailStyle = "strict" // RFC, and no quoted or unusual local parts
	EmailDNS    EmailStyle = "dns"    // the domain has MX or A records
	EmailSpoof  EmailStyle = "spoof"  // no mixing of scripts that look alike
	EmailFilter EmailStyle = "filter" // ASCII only, like PHP's FILTER_VALIDATE_EMAIL
	// EmailFilterUnicode is like EmailFilter but allows Unicode.
	EmailFilterUnicode EmailStyle = "filter_unicode"
)

var (
	strictLocal = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(\.[A-Za-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*$`)
	domainRe    = regexp.MustCompile(`^([\p{L}\p{N}]([\p{L}\p{N}-]{0,61}[\p{L}\p{N}])?\.)+[\p{L}]{2,63}$`)
)

// Email passes a bare email address such as "jane@example.com". With no
// styles it checks RFC syntax.
func Email(styles ...EmailStyle) Rule {
	if len(styles) == 0 {
		styles = []EmailStyle{EmailRFC}
	}
	return &funcRule{name: "email", fn: func(ctx context.Context, f Field) error {
		s, ok := f.Value.(string)
		if !ok || !validEmail(ctx, s, styles) {
			return Fail("email", nil)
		}
		return nil
	}}
}

func emailRule(p []string) (Rule, error) {
	styles := make([]EmailStyle, 0, len(p))
	for _, s := range p {
		switch st := EmailStyle(s); st {
		case EmailRFC, EmailStrict, EmailDNS, EmailSpoof, EmailFilter, EmailFilterUnicode:
			styles = append(styles, st)
		default:
			return nil, fmt.Errorf("unknown email style %q", s)
		}
	}
	return Email(styles...), nil
}

func validEmail(ctx context.Context, s string, styles []EmailStyle) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	local, domain := s[:at], s[at+1:]
	for _, st := range styles {
		switch st {
		case EmailStrict:
			if !strictLocal.MatchString(local) || !domainRe.MatchString(domain) {
				return false
			}
		case EmailFilter:
			if !isASCII(s) || !strictLocal.MatchString(local) || !domainRe.MatchString(domain) {
				return false
			}
		case EmailFilterUnicode:
			if !domainRe.MatchString(domain) {
				return false
			}
		case EmailSpoof:
			if mixedScripts(s) {
				return false
			}
		case EmailDNS:
			if !hasMailHost(ctx, domain) {
				return false
			}
		}
	}
	return true
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// mixedScripts reports whether s mixes letters from more than one script,
// the usual sign of a look-alike (homograph) address.
func mixedScripts(s string) bool {
	scripts := []*unicode.RangeTable{unicode.Latin, unicode.Cyrillic, unicode.Greek, unicode.Armenian, unicode.Arabic, unicode.Han, unicode.Hebrew}
	seen := -1
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		for i, t := range scripts {
			if unicode.Is(t, r) {
				if seen != -1 && seen != i {
					return true
				}
				seen = i
			}
		}
	}
	return false
}

func hasMailHost(ctx context.Context, domain string) bool {
	if mx, err := net.DefaultResolver.LookupMX(ctx, domain); err == nil && len(mx) > 0 {
		return true
	}
	hosts, err := net.DefaultResolver.LookupHost(ctx, domain)
	return err == nil && len(hosts) > 0
}

// --- URLs and identifiers ---------------------------------------------------------

var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*$`)

// URL passes absolute URLs. With protocols, the scheme must be one of them
// (url:http,https).
func URL(protocols ...string) Rule {
	return check("url", nil, func(f Field) bool {
		s, ok := f.Value.(string)
		if !ok || strings.ContainsAny(s, " \t\r\n") {
			return false
		}
		u, err := url.Parse(s)
		if err != nil || !schemeRe.MatchString(u.Scheme) || (u.Host == "" && u.Opaque == "") {
			return false
		}
		return len(protocols) == 0 || containsString(protocols, strings.ToLower(u.Scheme))
	})
}

// ActiveURL passes URLs (or bare host names) whose host resolves in DNS.
func ActiveURL() Rule {
	return &funcRule{name: "active_url", fn: func(ctx context.Context, f Field) error {
		s, ok := f.Value.(string)
		if !ok {
			return Fail("active_url", nil)
		}
		host := s
		if u, err := url.Parse(s); err == nil && u.Hostname() != "" {
			host = u.Hostname()
		}
		if addrs, err := net.DefaultResolver.LookupHost(ctx, host); err != nil || len(addrs) == 0 {
			return Fail("active_url", nil)
		}
		return nil
	}}
}

var (
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	ulidRe = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{25}$`)
	hexRe  = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
)

// UUID passes RFC 4122 UUIDs of any version.
func UUID() Rule {
	return stringCheck("uuid", nil, uuidRe.MatchString)
}

// UUIDVersion passes UUIDs of one version (1 to 8).
func UUIDVersion(version int) Rule {
	want := strconv.Itoa(version)[0]
	return stringCheck("uuid", nil, func(s string) bool { return uuidRe.MatchString(s) && s[14] == want })
}

// ULID passes ULIDs.
func ULID() Rule { return stringCheck("ulid", nil, ulidRe.MatchString) }

func ipCheck(name string, ok func(netip.Addr) bool) Rule {
	return stringCheck(name, nil, func(s string) bool {
		a, err := netip.ParseAddr(s)
		return err == nil && ok(a)
	})
}

// IP passes IPv4 and IPv6 addresses.
func IP() Rule { return ipCheck("ip", func(netip.Addr) bool { return true }) }

// IPv4 passes IPv4 addresses.
func IPv4() Rule { return ipCheck("ipv4", netip.Addr.Is4) }

// IPv6 passes IPv6 addresses.
func IPv6() Rule { return ipCheck("ipv6", netip.Addr.Is6) }

// MACAddress passes MAC addresses such as "00:1A:2B:3C:4D:5E".
func MACAddress() Rule {
	return stringCheck("mac_address", nil, func(s string) bool { _, err := net.ParseMAC(s); return err == nil })
}

// HexColor passes colours such as "#fff", "#ffcc00" or "#ffcc0080".
func HexColor() Rule { return stringCheck("hex_color", nil, hexRe.MatchString) }
