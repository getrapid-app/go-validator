package validator

import (
	"context"
	"strings"
	"time"
)

func registerDates(r *Registry) {
	r.Register("date", fixed(Date())).
		Register("date_format", atLeast(1, "a format", func(p []string) (Rule, error) { return DateFormat(p...), nil })).
		Register("after", oneField(After)).
		Register("after_or_equal", oneField(AfterOrEqual)).
		Register("before", oneField(Before)).
		Register("before_or_equal", oneField(BeforeOrEqual)).
		Register("date_equals", oneField(DateEquals)).
		Register("timezone", func(p []string) (Rule, error) { return Timezone(), nil })
}

// DateLayouts are the layouts tried, in order, when a rule needs to read a
// string as a date and the field has no date_format rule.
var DateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02",
	"2006/01/02 15:04:05",
	"02-01-2006",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC850,
	time.ANSIC,
	"January 2, 2006",
	"Jan 2, 2006",
	"2 January 2006",
	"2 Jan 2006",
}

// parseDate reads v as a time. Strings are parsed with layouts when given,
// otherwise with DateLayouts. Numbers are not dates, as in Laravel.
func parseDate(v any, layouts []string) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, !t.IsZero()
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	case string:
		s := strings.TrimSpace(t)
		if len(layouts) == 0 {
			layouts = DateLayouts
		}
		for _, l := range layouts {
			if d, err := time.Parse(l, s); err == nil {
				return d, true
			}
		}
	}
	return time.Time{}, false
}

// fieldLayouts returns the Go layouts of the field's date_format rule.
func fieldLayouts(f Field) []string {
	if r, ok := f.rule("date_format").(*dateFormatRule); ok {
		return r.layouts
	}
	return nil
}

// Date passes time.Time values and strings that parse as a date.
func Date() Rule {
	return check("date", nil, func(f Field) bool { _, ok := parseDate(f.Value, nil); return ok })
}

type dateFormatRule struct {
	formats []string // as written, for the message
	layouts []string // Go layouts
}

// DateFormat passes strings that exactly match one of the formats. Formats
// may be Go layouts ("2006-01-02") or PHP formats as used in Laravel
// ("Y-m-d").
func DateFormat(formats ...string) Rule {
	r := &dateFormatRule{formats: formats}
	for _, f := range formats {
		r.layouts = append(r.layouts, goLayout(f))
	}
	return r
}

func (*dateFormatRule) Name() string { return "date_format" }
func (r *dateFormatRule) Validate(_ context.Context, f Field) error {
	if s, ok := f.Value.(string); ok {
		for _, l := range r.layouts {
			if d, err := time.Parse(l, s); err == nil && d.Format(l) == s {
				return nil
			}
		}
	}
	return Fail("date_format", map[string]any{"format": strings.Join(r.formats, ", ")})
}

type dateCompareRule struct {
	name  string
	param string
	ok    func(a, b time.Time) bool
}

func (r dateCompareRule) Name() string { return r.name }
func (r dateCompareRule) Validate(_ context.Context, f Field) error {
	fail := Fail(r.name, map[string]any{"date": r.param})
	v, ok := parseDate(f.Value, fieldLayouts(f))
	if !ok {
		return fail
	}
	target, ok := relativeDate(r.param, f.Now())
	if !ok {
		target, ok = parseDate(r.param, nil)
	}
	if !ok {
		o := f.other(r.param)
		o.rules = f.rules
		target, ok = parseDate(o.Value, fieldLayouts(o))
		if ok {
			fail = Fail(r.name, map[string]any{"date": attrNames{r.param}})
		}
	}
	if !ok || !r.ok(v, target) {
		return fail
	}
	return nil
}

func relativeDate(s string, now time.Time) (time.Time, bool) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "now":
		return now, true
	case "today":
		return day, true
	case "tomorrow":
		return day.AddDate(0, 0, 1), true
	case "yesterday":
		return day.AddDate(0, 0, -1), true
	}
	return time.Time{}, false
}

// After passes dates after date, where date is a date string, "now",
// "today", "tomorrow", "yesterday", or the name of another field.
func After(date string) Rule {
	return dateCompareRule{"after", date, func(a, b time.Time) bool { return a.After(b) }}
}

// AfterOrEqual is like After, allowing the same instant.
func AfterOrEqual(date string) Rule {
	return dateCompareRule{"after_or_equal", date, func(a, b time.Time) bool { return !a.Before(b) }}
}

// Before passes dates before date (see After for what date may be).
func Before(date string) Rule {
	return dateCompareRule{"before", date, func(a, b time.Time) bool { return a.Before(b) }}
}

// BeforeOrEqual is like Before, allowing the same instant.
func BeforeOrEqual(date string) Rule {
	return dateCompareRule{"before_or_equal", date, func(a, b time.Time) bool { return !a.After(b) }}
}

// DateEquals passes dates equal to date (see After for what date may be).
func DateEquals(date string) Rule {
	return dateCompareRule{"date_equals", date, func(a, b time.Time) bool { return a.Equal(b) }}
}

// Timezone passes IANA time zone names such as "Africa/Cairo" or "UTC".
func Timezone() Rule {
	return check("timezone", nil, func(f Field) bool {
		s, ok := f.Value.(string)
		if !ok || s == "" || s == "Local" {
			return false
		}
		_, err := time.LoadLocation(s)
		return err == nil
	})
}

// goLayout converts a PHP date format ("Y-m-d H:i:s") to a Go layout. A
// string that already looks like a Go layout is returned unchanged.
func goLayout(format string) string {
	if strings.Contains(format, "2006") || strings.Contains(format, "15:04") || strings.Contains(format, "Jan") {
		return format
	}
	repl := map[byte]string{
		'd': "02", 'j': "2", 'D': "Mon", 'l': "Monday",
		'm': "01", 'n': "1", 'M': "Jan", 'F': "January",
		'Y': "2006", 'y': "06",
		'H': "15", 'G': "15", 'h': "03", 'g': "3", 'i': "04", 's': "05",
		'A': "PM", 'a': "pm", 'u': "000000", 'v': "000",
		'e': "MST", 'T': "MST", 'P': "-07:00", 'O': "-0700", 'p': "Z07:00",
		'c': time.RFC3339, 'U': "",
	}
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c == '\\' && i+1 < len(format) {
			i++
			b.WriteByte(format[i])
			continue
		}
		if r, ok := repl[c]; ok {
			b.WriteString(r)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
