package validator

// DefaultMaxErrors is how many fields may fail before validation stops.
// Once that many fields have errors, the remaining fields are not checked
// and the errors so far are returned, so a request with thousands of bad
// items costs no more than one with a hundred. Change it at program start,
// or per Validator or call with WithMaxErrors or StopOnFirstFailure. Zero
// or a negative value means no limit.
var DefaultMaxErrors = 100

// WithMaxErrors stops validation once n fields have failed, in place of
// DefaultMaxErrors. Pass a negative value to check every field.
func WithMaxErrors(n int) Option {
	return func(v *Validator) {
		if n == 0 {
			n = -1
		}
		v.maxErrors = n
	}
}

// StopOnFirstFailure stops validation at the first field that fails, like
// Laravel's $stopOnFirstFailure. The field still gets all of its own
// messages unless it uses bail.
func StopOnFirstFailure() Option { return WithMaxErrors(1) }

// FirstFailureStopper is implemented by request types that stop at the
// first failing field, like a Laravel form request with
// $stopOnFirstFailure = true.
type FirstFailureStopper interface {
	StopOnFirstFailure() bool
}

// errorLimit returns how many fields may fail, or 0 for no limit.
func (v *Validator) errorLimit() int {
	n := v.maxErrors
	if n == 0 {
		n = DefaultMaxErrors
	}
	return max(n, 0)
}
