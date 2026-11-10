package validator_test

import (
	"errors"
	"io"
	"testing"

	validator "github.com/getrapid-app/go-validator"
)

// sizedFile reports a size without holding the data. Opening it fails, so a
// test also shows the content is never read.
type sizedFile struct {
	name string
	size int64
}

func (s sizedFile) FileName() string             { return s.name }
func (s sizedFile) FileSize() int64              { return s.size }
func (s sizedFile) Open() (io.ReadCloser, error) { return nil, errors.New("must not be opened") }

const mb = 1 << 20

func TestDefaultMaxFileSize(t *testing.T) {
	big, small := sizedFile{"big.bin", 11 * mb}, sizedFile{"small.bin", 1 * mb}
	cases := []struct {
		name  string
		data  map[string]any
		rules validator.Rules
		opts  []validator.Option
		want  map[string]string // field -> first message; empty means valid
	}{
		{"default limit", map[string]any{"doc": big}, validator.Rules{"doc": "required"}, nil,
			map[string]string{"doc": "The doc field must not be greater than 10240 kilobytes."}},
		{"under default", map[string]any{"doc": sizedFile{"a", 10 * mb}}, validator.Rules{"doc": "required"}, nil, nil},
		{"own max raises it", map[string]any{"doc": big}, validator.Rules{"doc": "max:20480"}, nil, nil},
		{"own max lowers it", map[string]any{"doc": sizedFile{"a", 2 * mb}}, validator.Rules{"doc": "max:1024"}, nil,
			map[string]string{"doc": "The doc field must not be greater than 1024 kilobytes."}},
		{"between counts", map[string]any{"doc": big}, validator.Rules{"doc": "between:1,20480"}, nil, nil},
		{"min does not", map[string]any{"doc": big}, validator.Rules{"doc": "min:1"}, nil,
			map[string]string{"doc": "The doc field must not be greater than 10240 kilobytes."}},
		{"option lowers it", map[string]any{"doc": sizedFile{"a", 2 * mb}}, validator.Rules{"doc": "required"},
			[]validator.Option{validator.WithMaxFileSize(1 * mb)},
			map[string]string{"doc": "The doc field must not be greater than 1024 kilobytes."}},
		{"option turns it off", map[string]any{"doc": big}, validator.Rules{"doc": "required"},
			[]validator.Option{validator.WithMaxFileSize(-1)}, nil},
		{"files in a list", map[string]any{"photos": []any{small, big}}, validator.Rules{"photos": "array|max:5"}, nil,
			map[string]string{"photos.1": "The photos.1 field must not be greater than 10240 kilobytes."}},
		{"wildcard skips other rules", map[string]any{"photos": []any{small, big}}, validator.Rules{"photos.*": "required|extensions:bin"}, nil,
			map[string]string{"photos.1": "The photos.1 field must not be greater than 10240 kilobytes."}},
	}
	for _, tc := range cases {
		err := validator.Validate(ctx, tc.data, tc.rules, tc.opts...)
		if len(tc.want) == 0 {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		bag := mustErrors(t, err)
		if len(bag.Fields()) != len(tc.want) {
			t.Errorf("%s: failed fields %v, want %v", tc.name, bag.Fields(), tc.want)
		}
		for field, msg := range tc.want {
			if got := bag.First(field); got != msg {
				t.Errorf("%s: %s = %q, want %q", tc.name, field, got, msg)
			}
			if n := len(bag.Get(field)); n != 1 {
				t.Errorf("%s: %s has %d messages, want 1", tc.name, field, n)
			}
		}
	}
}

func TestDefaultMaxFileSizeVariable(t *testing.T) {
	old := validator.DefaultMaxFileSize
	defer func() { validator.DefaultMaxFileSize = old }()
	validator.DefaultMaxFileSize = 512 << 10
	err := validator.Validate(ctx, map[string]any{"doc": sizedFile{"a", 1 * mb}}, validator.Rules{"doc": "required"})
	if got := mustErrors(t, err).First("doc"); got != "The doc field must not be greater than 512 kilobytes." {
		t.Fatalf("got %q", got)
	}
	validator.DefaultMaxFileSize = 0
	if err := validator.Validate(ctx, map[string]any{"doc": sizedFile{"a", 100 * mb}}, validator.Rules{"doc": "required"}); err != nil {
		t.Fatalf("no limit: %v", err)
	}
}

func TestDefaultMaxFileSizeInStructs(t *testing.T) {
	type upload struct {
		Avatar validator.File `json:"avatar" validate:"required"`
		Doc    validator.File `json:"doc" validate:"required|max:20480"`
	}
	err := validator.Struct(ctx, upload{Avatar: sizedFile{"a.png", 11 * mb}, Doc: sizedFile{"d.pdf", 15 * mb}})
	bag := mustErrors(t, err)
	if f := bag.Fields(); len(f) != 1 || f[0] != "avatar" {
		t.Fatalf("failed fields %v, want [avatar]", f)
	}
}
