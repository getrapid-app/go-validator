package validator

import (
	"math"
	"strconv"
)

// DefaultMaxFileSize is the largest file, in bytes, that any field accepts
// when its rules set no size limit of their own. A field's max, size,
// between, lt or lte rule replaces it, so "image|max:20480" allows 20 MB
// even though the default is lower. Change it at program start, or per
// Validator with WithMaxFileSize. Zero or a negative value means no default
// limit.
//
// The check runs before the field's other rules, so an oversized file is
// rejected without its content being read.
var DefaultMaxFileSize int64 = 10 << 20

// WithMaxFileSize sets the default largest file size in bytes for this
// Validator, in place of DefaultMaxFileSize. Pass a negative value to turn
// the default limit off.
func WithMaxFileSize(bytes int64) Option {
	return func(v *Validator) {
		if bytes == 0 {
			bytes = -1
		}
		v.maxFile = bytes
	}
}

// fileLimit returns the default largest file size, or 0 for none.
func (v *Validator) fileLimit() int64 {
	n := v.maxFile
	if n == 0 {
		n = DefaultMaxFileSize
	}
	return max(n, 0)
}

// sizeLimitRules are the rules that put an upper bound on a file's size.
var sizeLimitRules = map[string]bool{"max": true, "size": true, "between": true, "lt": true, "lte": true}

// checkFileSizes applies the default file size limit to the value of one
// field: a file, or the files directly inside a list or map. It reports
// whether the field itself failed, in which case its rules are skipped.
func (v *Validator) checkFileSizes(f Field, spec *fieldSpec, limit int64, bag *Errors) bool {
	switch t := f.Value.(type) {
	case File:
		if spec.sizeLimited || t.FileSize() <= limit {
			return false
		}
		bag.Add(f.Path, v.fileTooLarge(f, limit))
		return true
	case []any:
		for i, x := range t {
			if file, ok := x.(File); ok && file.FileSize() > limit {
				item := f
				item.Path, item.Pattern, item.Value = f.Path+"."+strconv.Itoa(i), f.Pattern+".*", x
				bag.Add(item.Path, v.fileTooLarge(item, limit))
			}
		}
	case map[string]any:
		for k, x := range t {
			if file, ok := x.(File); ok && file.FileSize() > limit {
				item := f
				item.Path, item.Pattern, item.Value = f.Path+"."+k, f.Pattern+".*", x
				bag.Add(item.Path, v.fileTooLarge(item, limit))
			}
		}
	}
	return false
}

func (v *Validator) fileTooLarge(f Field, limit int64) string {
	kb := math.Floor(float64(limit) / 1024)
	return v.message(f, "max", &Failure{Key: "max.file", Params: map[string]any{"max": kb}})
}
