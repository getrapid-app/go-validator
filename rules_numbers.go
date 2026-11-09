package validator

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func registerNumbers(r *Registry) {
	r.Register("size", oneNumber(Size)).
		Register("min", oneNumber(Min)).
		Register("max", oneNumber(Max)).
		Register("between", twoNumbers(Between)).
		Register("gt", oneField(GreaterThan)).
		Register("gte", oneField(GreaterThanOrEqual)).
		Register("lt", oneField(LessThan)).
		Register("lte", oneField(LessThanOrEqual)).
		Register("digits", oneInt(Digits)).
		Register("digits_between", twoInts(DigitsBetween)).
		Register("min_digits", oneInt(MinDigits)).
		Register("max_digits", oneInt(MaxDigits)).
		Register("decimal", atLeast(1, "a number of decimal places", func(p []string) (Rule, error) {
			lo, err := strconv.Atoi(p[0])
			if err != nil {
				return nil, fmt.Errorf("needs a whole number, got %q", p[0])
			}
			hi := lo
			if len(p) > 1 {
				if hi, err = strconv.Atoi(p[1]); err != nil {
					return nil, fmt.Errorf("needs a whole number, got %q", p[1])
				}
			}
			return Decimal(lo, hi), nil
		})).
		Register("multiple_of", atLeast(1, "a number", func(p []string) (Rule, error) {
			if _, ok := new(big.Rat).SetString(p[0]); !ok {
				return nil, fmt.Errorf("needs a number, got %q", p[0])
			}
			return MultipleOf(p[0]), nil
		}))
}

// sizeOf measures a value the way Laravel's size rules do: numbers by value,
// strings by character count (or by value when the field also has the
// numeric or integer rule), arrays by length and files in kilobytes.
func sizeOf(f Field) (size float64, kind string, ok bool) {
	switch v := f.Value.(type) {
	case string:
		if f.HasRule("numeric") || f.HasRule("integer") || f.HasRule("decimal") {
			n, ok := toFloat(v)
			return n, "numeric", ok
		}
		return float64(utf8.RuneCountInString(v)), "string", true
	case []any:
		return float64(len(v)), "array", true
	case map[string]any:
		return float64(len(v)), "array", true
	case File:
		return float64(v.FileSize()) / 1024, "file", true
	}
	n, ok := toFloat(f.Value)
	return n, "numeric", ok
}

type sizeRule struct {
	name   string
	lo, hi float64
	hasLo  bool
	hasHi  bool
}

func (r sizeRule) Name() string { return r.name }
func (r sizeRule) Validate(_ context.Context, f Field) error {
	size, kind, ok := sizeOf(f)
	if !ok {
		return Fail(r.name+".numeric", r.params())
	}
	if (r.hasLo && size < r.lo) || (r.hasHi && size > r.hi) {
		return Fail(r.name+"."+kind, r.params())
	}
	return nil
}

// params is built only on failure, so passing values cost no allocation.
func (r sizeRule) params() map[string]any {
	return map[string]any{"min": r.lo, "max": r.hi, "size": r.lo}
}

// Size fails unless the value's size is exactly n: the number itself, the
// number of characters, items, or kilobytes for files.
func Size(n float64) Rule {
	return sizeRule{name: "size", lo: n, hi: n, hasLo: true, hasHi: true}
}

// Min fails when the value's size is below n.
func Min(n float64) Rule { return sizeRule{name: "min", lo: n, hasLo: true} }

// Max fails when the value's size is above n.
func Max(n float64) Rule { return sizeRule{name: "max", hi: n, hasHi: true} }

// Between fails when the value's size is outside [lo, hi].
func Between(lo, hi float64) Rule {
	return sizeRule{name: "between", lo: lo, hi: hi, hasLo: true, hasHi: true}
}

// --- comparisons with another field or a number --------------------------------

type compareRule struct {
	name  string
	other string
	ok    func(a, b float64) bool
}

func (r compareRule) Name() string { return r.name }
func (r compareRule) Validate(_ context.Context, f Field) error {
	size, kind, ok := sizeOf(f)
	if !ok {
		return Fail(r.name+".numeric", map[string]any{"value": r.other})
	}
	var target float64
	o := f.other(r.other)
	if o.Present {
		// Compare with the other field measured the same way as this one.
		o.rules = f.rules
		osize, okind, ok := sizeOf(o)
		if !ok || okind != kind {
			return Fail(r.name+"."+kind, map[string]any{"value": r.other})
		}
		target = osize
	} else if n, err := strconv.ParseFloat(r.other, 64); err == nil {
		target = n
	} else {
		return Fail(r.name+"."+kind, map[string]any{"value": r.other})
	}
	if !r.ok(size, target) {
		return Fail(r.name+"."+kind, map[string]any{"value": target})
	}
	return nil
}

// GreaterThan fails unless the value's size is greater than another field's
// size, or than a number when other is numeric and not a field.
func GreaterThan(other string) Rule {
	return compareRule{"gt", other, func(a, b float64) bool { return a > b }}
}

// GreaterThanOrEqual is like GreaterThan, allowing equality.
func GreaterThanOrEqual(other string) Rule {
	return compareRule{"gte", other, func(a, b float64) bool { return a >= b }}
}

// LessThan fails unless the value's size is less than another field's size,
// or than a number.
func LessThan(other string) Rule {
	return compareRule{"lt", other, func(a, b float64) bool { return a < b }}
}

// LessThanOrEqual is like LessThan, allowing equality.
func LessThanOrEqual(other string) Rule {
	return compareRule{"lte", other, func(a, b float64) bool { return a <= b }}
}

// --- digits -----------------------------------------------------------------------

var onlyDigits = regexp.MustCompile(`^[0-9]+$`)

func digitsOf(v any) (string, bool) {
	s, ok := textOf(v)
	return s, ok && onlyDigits.MatchString(s)
}

func digitsRule(name string, params map[string]any, ok func(n int) bool) Rule {
	return check(name, params, func(f Field) bool {
		s, isDigits := digitsOf(f.Value)
		return isDigits && ok(len(s))
	})
}

// Digits passes integers with exactly n digits.
func Digits(n int) Rule {
	return digitsRule("digits", map[string]any{"digits": n}, func(l int) bool { return l == n })
}

// DigitsBetween passes integers with lo to hi digits.
func DigitsBetween(lo, hi int) Rule {
	return digitsRule("digits_between", map[string]any{"min": lo, "max": hi}, func(l int) bool { return l >= lo && l <= hi })
}

// MinDigits passes integers with at least n digits.
func MinDigits(n int) Rule {
	return digitsRule("min_digits", map[string]any{"min": n}, func(l int) bool { return l >= n })
}

// MaxDigits passes integers with at most n digits.
func MaxDigits(n int) Rule {
	return digitsRule("max_digits", map[string]any{"max": n}, func(l int) bool { return l <= n })
}

// Decimal passes numbers with between lo and hi decimal places. Note that
// JSON numbers lose trailing zeros (1.50 decodes as 1.5); send the value as
// a string when the exact number of places matters.
func Decimal(lo, hi int) Rule {
	param := strconv.Itoa(lo)
	if hi != lo {
		param += "-" + strconv.Itoa(hi)
	}
	return check("decimal", map[string]any{"decimal": param}, func(f Field) bool {
		s, ok := textOf(f.Value)
		if !ok {
			return false
		}
		if _, isNum := toFloat(s); !isNum {
			return false
		}
		_, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
		if strings.ContainsAny(frac, "eE") {
			return false
		}
		return len(frac) >= lo && len(frac) <= hi
	})
}

// MultipleOf passes numbers that are an exact multiple of n, given as text
// so decimals such as "0.1" are exact.
func MultipleOf(n string) Rule {
	div, ok := new(big.Rat).SetString(n)
	if !ok {
		panic(fmt.Sprintf("validator: MultipleOf needs a number, got %q", n))
	}
	return check("multiple_of", map[string]any{"value": n}, func(f Field) bool {
		s, ok := textOf(f.Value)
		if !ok {
			return false
		}
		v, ok := new(big.Rat).SetString(strings.TrimSpace(s))
		if !ok {
			return false
		}
		if div.Sign() == 0 {
			return false
		}
		return new(big.Rat).Quo(v, div).IsInt()
	})
}
