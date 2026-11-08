package validator

import (
	"context"
	"errors"
)

// PresenceVerifier looks values up in your data store for the exists and
// unique rules. Implement it once for your database (SQL, Mongo, an API, a
// map in tests) and pass it with WithPresenceVerifier.
type PresenceVerifier interface {
	// Count returns how many records match q.
	Count(ctx context.Context, q PresenceQuery) (int64, error)
}

// PresenceVerifierFunc adapts a function to PresenceVerifier.
type PresenceVerifierFunc func(ctx context.Context, q PresenceQuery) (int64, error)

func (fn PresenceVerifierFunc) Count(ctx context.Context, q PresenceQuery) (int64, error) {
	return fn(ctx, q)
}

// PresenceQuery describes one lookup: records in Table whose Column equals
// Value, except the record whose IgnoreColumn equals Ignore (for unique on
// updates), and matching every Where condition.
type PresenceQuery struct {
	Table        string
	Column       string
	Value        any
	Ignore       any
	IgnoreColumn string
	Where        map[string]any
}

// ErrNoPresenceVerifier is returned when exists or unique runs without a
// verifier configured.
var ErrNoPresenceVerifier = errors.New("validator: exists/unique need WithPresenceVerifier")

func registerDB(r *Registry) {
	r.Register("exists", atLeast(1, "a table", func(p []string) (Rule, error) {
		e := Exists(p[0])
		if len(p) > 1 {
			e.Column(p[1])
		}
		return e, nil
	})).
		Register("unique", atLeast(1, "a table", func(p []string) (Rule, error) {
			u := Unique(p[0])
			if len(p) > 1 {
				u.Column(p[1])
			}
			if len(p) > 2 && p[2] != "" && p[2] != "NULL" {
				col := "id"
				if len(p) > 3 {
					col = p[3]
				}
				u.Ignore(p[2], col)
			}
			return u, nil
		}))
}

// DBRule is the exists or unique rule. Build it with Exists or Unique and
// refine it fluently.
type DBRule struct {
	name string
	q    PresenceQuery
}

// Exists passes values that exist in table. The column defaults to the
// field's last path segment.
//
//	validator.Exists("users").Column("email").Where("active", true)
func Exists(table string) *DBRule { return &DBRule{name: "exists", q: PresenceQuery{Table: table}} }

// Unique passes values that do not exist in table yet.
//
//	validator.Unique("users").Column("email").Ignore(user.ID, "id")
func Unique(table string) *DBRule { return &DBRule{name: "unique", q: PresenceQuery{Table: table}} }

// Column sets the column to look in.
func (d *DBRule) Column(c string) *DBRule { d.q.Column = c; return d }

// Where adds an extra condition.
func (d *DBRule) Where(column string, value any) *DBRule {
	if d.q.Where == nil {
		d.q.Where = map[string]any{}
	}
	d.q.Where[column] = value
	return d
}

// Ignore skips the record whose column (default "id") equals value, so a
// record being updated does not conflict with itself.
func (d *DBRule) Ignore(value any, column ...string) *DBRule {
	d.q.Ignore = value
	d.q.IgnoreColumn = "id"
	if len(column) > 0 {
		d.q.IgnoreColumn = column[0]
	}
	return d
}

func (d *DBRule) Name() string { return d.name }

func (d *DBRule) Validate(ctx context.Context, f Field) error {
	pv := f.Presence()
	if pv == nil {
		return Abort(ErrNoPresenceVerifier)
	}
	q := d.q
	if q.Column == "" {
		q.Column = lastSegment(f.Path)
	}
	vals := []any{f.Value}
	if list, ok := f.Value.([]any); ok && d.name == "exists" {
		vals = list
	}
	for _, v := range vals {
		q.Value = v
		n, err := pv.Count(ctx, q)
		if err != nil {
			return Abort(err)
		}
		if (d.name == "exists" && n == 0) || (d.name == "unique" && n > 0) {
			return Fail(d.name, nil)
		}
	}
	return nil
}

func lastSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i+1:]
		}
	}
	return path
}
