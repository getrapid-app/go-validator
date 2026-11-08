package validator

import (
	"context"
	"errors"
	"unicode"
	"unicode/utf8"
)

// CurrentPasswordChecker checks a password against the signed-in user's,
// for the current_password rule. Find the user from ctx.
type CurrentPasswordChecker interface {
	CheckCurrentPassword(ctx context.Context, password string) (bool, error)
}

// CurrentPasswordFunc adapts a function to CurrentPasswordChecker.
type CurrentPasswordFunc func(ctx context.Context, password string) (bool, error)

func (fn CurrentPasswordFunc) CheckCurrentPassword(ctx context.Context, pw string) (bool, error) {
	return fn(ctx, pw)
}

// ErrNoPasswordChecker is returned when current_password runs without a
// checker configured.
var ErrNoPasswordChecker = errors.New("validator: current_password needs WithCurrentPassword")

func registerPassword(r *Registry) {
	r.Register("current_password", fixed(CurrentPassword()))
}

// CurrentPassword passes the signed-in user's current password, checked by
// the CurrentPasswordChecker set with WithCurrentPassword.
func CurrentPassword() Rule {
	return &funcRule{name: "current_password", fn: func(ctx context.Context, f Field) error {
		c := f.validator().password
		if c == nil {
			return Abort(ErrNoPasswordChecker)
		}
		s, _ := f.Value.(string)
		ok, err := c.CheckCurrentPassword(ctx, s)
		if err != nil {
			return Abort(err)
		}
		if !ok {
			return Fail("current_password", nil)
		}
		return nil
	}}
}

// BreachChecker reports whether a password appeared in a data breach, for
// PasswordRule.Uncompromised. A typical implementation queries the Have I
// Been Pwned range API.
type BreachChecker interface {
	Breached(ctx context.Context, password string) (bool, error)
}

// PasswordRule is Laravel's Password rule object, built fluently:
//
//	validator.Password().Min(12).MixedCase().Numbers().Symbols()
type PasswordRule struct {
	min, max                         int
	letters, mixed, numbers, symbols bool
	breach                           BreachChecker
}

// Password starts a password rule requiring at least 8 characters.
func Password() *PasswordRule { return &PasswordRule{min: 8} }

func (p *PasswordRule) Min(n int) *PasswordRule  { p.min = n; return p }
func (p *PasswordRule) Max(n int) *PasswordRule  { p.max = n; return p }
func (p *PasswordRule) Letters() *PasswordRule   { p.letters = true; return p }
func (p *PasswordRule) MixedCase() *PasswordRule { p.mixed = true; return p }
func (p *PasswordRule) Numbers() *PasswordRule   { p.numbers = true; return p }
func (p *PasswordRule) Symbols() *PasswordRule   { p.symbols = true; return p }

// Uncompromised rejects passwords that c reports as breached.
func (p *PasswordRule) Uncompromised(c BreachChecker) *PasswordRule { p.breach = c; return p }

func (*PasswordRule) Name() string { return "password" }

func (p *PasswordRule) Validate(ctx context.Context, f Field) error {
	s, ok := f.Value.(string)
	if !ok {
		return Fail("string", nil)
	}
	n := utf8.RuneCountInString(s)
	if n < p.min {
		return Fail("min.string", map[string]any{"min": p.min})
	}
	if p.max > 0 && n > p.max {
		return Fail("max.string", map[string]any{"max": p.max})
	}
	var lower, upper, letter, digit, symbol bool
	for _, r := range s {
		switch {
		case unicode.IsLower(r):
			lower, letter = true, true
		case unicode.IsUpper(r):
			upper, letter = true, true
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsNumber(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r):
			symbol = true
		}
	}
	switch {
	case p.letters && !letter:
		return Fail("password.letters", nil)
	case p.mixed && !(lower && upper):
		return Fail("password.mixed", nil)
	case p.numbers && !digit:
		return Fail("password.numbers", nil)
	case p.symbols && !symbol:
		return Fail("password.symbols", nil)
	}
	if p.breach != nil {
		breached, err := p.breach.Breached(ctx, s)
		if err != nil {
			return Abort(err)
		}
		if breached {
			return Fail("password.uncompromised", nil)
		}
	}
	return nil
}
