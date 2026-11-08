package validator

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"  // register decoders for the dimensions rule
	_ "image/jpeg" // ...
	_ "image/png"  // ...
	"strconv"
	"strings"
)

func registerFiles(r *Registry) {
	r.Register("file", fixed(FileRule())).
		Register("image", func(p []string) (Rule, error) { return Image(containsString(p, "allow_svg")), nil }).
		Register("mimes", atLeast(1, "at least one extension", func(p []string) (Rule, error) { return Mimes(p...), nil })).
		Register("mimetypes", atLeast(1, "at least one MIME type", func(p []string) (Rule, error) { return MimeTypes(p...), nil })).
		Register("extensions", atLeast(1, "at least one extension", func(p []string) (Rule, error) { return Extensions(p...), nil })).
		Register("dimensions", func(p []string) (Rule, error) {
			d := Dimensions()
			for _, kv := range p {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					return nil, fmt.Errorf("needs key=value constraints, got %q", kv)
				}
				if k == "ratio" {
					ratio, err := parseRatio(v)
					if err != nil {
						return nil, err
					}
					d.Ratio(ratio)
					continue
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("needs a whole number for %s, got %q", k, v)
				}
				switch k {
				case "width":
					d.Width(n)
				case "height":
					d.Height(n)
				case "min_width":
					d.MinWidth(n)
				case "max_width":
					d.MaxWidth(n)
				case "min_height":
					d.MinHeight(n)
				case "max_height":
					d.MaxHeight(n)
				default:
					return nil, fmt.Errorf("unknown constraint %q", k)
				}
			}
			return d, nil
		})
}

func fileCheck(name string, params map[string]any, ok func(f File) (bool, error)) Rule {
	return &funcRule{name: name, fn: func(_ context.Context, fl Field) error {
		file, isFile := fl.Value.(File)
		if !isFile {
			return Fail(name, params)
		}
		passed, err := ok(file)
		if err != nil {
			return Abort(fmt.Errorf("validator: reading file %q: %w", file.FileName(), err))
		}
		if !passed {
			return Fail(name, params)
		}
		return nil
	}}
}

// FileRule passes uploaded files (values implementing File). It is named
// FileRule because File is the interface.
func FileRule() Rule {
	return fileCheck("file", nil, func(File) (bool, error) { return true, nil })
}

// Image passes files whose content is jpeg, png, gif, bmp or webp, and svg
// when allowSVG is given and true.
func Image(allowSVG ...bool) Rule {
	types := []string{"image/jpeg", "image/png", "image/gif", "image/bmp", "image/webp"}
	if len(allowSVG) > 0 && allowSVG[0] {
		types = append(types, "image/svg+xml")
	}
	return fileCheck("image", nil, func(f File) (bool, error) {
		mt, err := sniff(f)
		return containsString(types, mt), err
	})
}

// Mimes passes files whose detected content type maps to one of the
// extensions, as Laravel's mimes:jpg,png does. The client's file name is
// not trusted.
func Mimes(extensions ...string) Rule {
	exts := lowerAll(extensions)
	return fileCheck("mimes", map[string]any{"values": extensions}, func(f File) (bool, error) {
		mt, err := sniff(f)
		if err != nil {
			return false, err
		}
		for _, e := range mimeExtensions[mt] {
			if containsString(exts, e) {
				return true, nil
			}
		}
		return false, nil
	})
}

// MimeTypes passes files whose detected content type is one of types.
// A type may end in "/*", as in "image/*".
func MimeTypes(types ...string) Rule {
	return fileCheck("mimetypes", map[string]any{"values": types}, func(f File) (bool, error) {
		mt, err := sniff(f)
		if err != nil {
			return false, err
		}
		for _, t := range types {
			if t == mt || (strings.HasSuffix(t, "/*") && strings.HasPrefix(mt, strings.TrimSuffix(t, "*"))) {
				return true, nil
			}
		}
		return false, nil
	})
}

// Extensions passes files whose name ends in one of the extensions. Combine
// it with Mimes or MimeTypes, since the name comes from the client.
func Extensions(extensions ...string) Rule {
	exts := lowerAll(extensions)
	return fileCheck("extensions", map[string]any{"values": extensions}, func(f File) (bool, error) {
		return containsString(exts, fileExtension(f)), nil
	})
}

func lowerAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(strings.TrimPrefix(s, "."))
	}
	return out
}

// DimensionsRule checks an image's size in pixels. Build it with
// Dimensions().MinWidth(100).Ratio(3.0/2) and so on.
type DimensionsRule struct {
	width, height                            int
	minWidth, maxWidth, minHeight, maxHeight int
	ratio                                    float64
}

// Dimensions starts an image dimensions rule with no constraints.
func Dimensions() *DimensionsRule { return &DimensionsRule{} }

func (d *DimensionsRule) Width(n int) *DimensionsRule     { d.width = n; return d }
func (d *DimensionsRule) Height(n int) *DimensionsRule    { d.height = n; return d }
func (d *DimensionsRule) MinWidth(n int) *DimensionsRule  { d.minWidth = n; return d }
func (d *DimensionsRule) MaxWidth(n int) *DimensionsRule  { d.maxWidth = n; return d }
func (d *DimensionsRule) MinHeight(n int) *DimensionsRule { d.minHeight = n; return d }
func (d *DimensionsRule) MaxHeight(n int) *DimensionsRule { d.maxHeight = n; return d }

// Ratio requires width divided by height to equal r, such as 3.0/2.
func (d *DimensionsRule) Ratio(r float64) *DimensionsRule { d.ratio = r; return d }

func (*DimensionsRule) Name() string { return "dimensions" }

func (d *DimensionsRule) Validate(_ context.Context, fl Field) error {
	file, ok := fl.Value.(File)
	if !ok {
		return Fail("dimensions", nil)
	}
	rc, err := file.Open()
	if err != nil {
		return Abort(err)
	}
	defer rc.Close()
	cfg, _, err := image.DecodeConfig(rc)
	if err != nil {
		return Fail("dimensions", nil)
	}
	w, h := cfg.Width, cfg.Height
	bad := (d.width > 0 && w != d.width) || (d.height > 0 && h != d.height) ||
		(d.minWidth > 0 && w < d.minWidth) || (d.maxWidth > 0 && w > d.maxWidth) ||
		(d.minHeight > 0 && h < d.minHeight) || (d.maxHeight > 0 && h > d.maxHeight)
	if d.ratio > 0 && h > 0 {
		diff := float64(w)/float64(h) - d.ratio
		if diff < 0 {
			diff = -diff
		}
		bad = bad || diff > 1.0/float64(maxInt(w, h)+1)
	}
	if bad {
		return Fail("dimensions", nil)
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseRatio(s string) (float64, error) {
	if a, b, ok := strings.Cut(s, "/"); ok {
		x, err1 := strconv.ParseFloat(a, 64)
		y, err2 := strconv.ParseFloat(b, 64)
		if err1 != nil || err2 != nil || y == 0 {
			return 0, fmt.Errorf("invalid ratio %q", s)
		}
		return x / y, nil
	}
	r, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ratio %q", s)
	}
	return r, nil
}
