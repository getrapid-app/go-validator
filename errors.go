package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Errors holds failure messages per field, in the order they were added.
// Its JSON form matches Laravel's: {"email": ["The email field is required."]}.
type Errors struct {
	order []string
	msgs  map[string][]string
}

// NewErrors returns an empty error bag.
func NewErrors() *Errors { return &Errors{msgs: map[string][]string{}} }

// Add appends a message for field.
func (e *Errors) Add(field, message string) {
	if e.msgs == nil {
		e.msgs = map[string][]string{}
	}
	if _, ok := e.msgs[field]; !ok {
		e.order = append(e.order, field)
	}
	e.msgs[field] = append(e.msgs[field], message)
}

// Has reports whether field has at least one message.
func (e *Errors) Has(field string) bool { return len(e.msgs[field]) > 0 }

// Get returns all messages for field.
func (e *Errors) Get(field string) []string { return e.msgs[field] }

// First returns the first message for field, or "".
func (e *Errors) First(field string) string {
	if m := e.msgs[field]; len(m) > 0 {
		return m[0]
	}
	return ""
}

// Fields returns the fields that have messages, in insertion order.
func (e *Errors) Fields() []string { return append([]string(nil), e.order...) }

// All returns a copy of every message, keyed by field.
func (e *Errors) All() map[string][]string {
	out := make(map[string][]string, len(e.msgs))
	for k, v := range e.msgs {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// Len returns the number of fields with messages.
func (e *Errors) Len() int { return len(e.order) }

// IsEmpty reports whether the bag has no messages.
func (e *Errors) IsEmpty() bool { return len(e.order) == 0 }

// MarshalJSON writes the fields in insertion order.
func (e *Errors) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range e.order {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(e.msgs[k])
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ValidationError is returned when at least one field fails.
type ValidationError struct {
	Errors *Errors
}

func (e *ValidationError) Error() string {
	if e.Errors.IsEmpty() {
		return "validation failed"
	}
	first := e.Errors.First(e.Errors.order[0])
	if n := e.Errors.Len() - 1; n > 0 {
		return fmt.Sprintf("%s (and %d more field(s) with errors)", first, n)
	}
	return first
}

// ErrorsOf returns the error bag carried by err, if err is (or wraps) a
// ValidationError.
func ErrorsOf(err error) (*Errors, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Errors, true
	}
	return nil, false
}
