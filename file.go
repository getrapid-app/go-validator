package validator

import (
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
)

// File is an uploaded file. The file rules (file, image, mimes, mimetypes,
// extensions, dimensions) and the size rules (in kilobytes) work on any
// value that implements it. *multipart.FileHeader values in the input are
// converted to a File automatically.
type File interface {
	// FileName is the client-supplied name, such as "avatar.png".
	FileName() string
	// FileSize is the size in bytes.
	FileSize() int64
	// Open returns the file's content.
	Open() (io.ReadCloser, error)
}

type multipartFile struct{ h *multipart.FileHeader }

func (m multipartFile) FileName() string             { return m.h.Filename }
func (m multipartFile) FileSize() int64              { return m.h.Size }
func (m multipartFile) Open() (io.ReadCloser, error) { return m.h.Open() }

// FromMultipart wraps a multipart file header as a File.
func FromMultipart(h *multipart.FileHeader) File { return multipartFile{h} }

var fileType = reflect.TypeOf((*File)(nil)).Elem()

func asFile(rv reflect.Value) (File, bool) {
	if !rv.IsValid() || !rv.CanInterface() {
		return nil, false
	}
	switch v := rv.Interface().(type) {
	case *multipart.FileHeader:
		return multipartFile{v}, true
	case File:
		return v, true
	}
	return nil, false
}

// sniff returns the detected MIME type of f's content.
func sniff(f File) (string, error) {
	r, err := f.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	buf := make([]byte, 512)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	ct := http.DetectContentType(buf[:n])
	// DetectContentType has no SVG signature; recognise it by content.
	if strings.HasPrefix(ct, "text/") && strings.Contains(strings.ToLower(string(buf[:n])), "<svg") {
		ct = "image/svg+xml"
	}
	mt, _, _ := strings.Cut(ct, ";")
	return strings.TrimSpace(mt), nil
}

func fileExtension(f File) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(f.FileName())), ".")
}

// mimeExtensions maps detected MIME types to the extensions the mimes rule
// accepts for them, following Laravel's guesser for common types.
var mimeExtensions = map[string][]string{
	"image/jpeg":                    {"jpg", "jpeg", "jpe"},
	"image/png":                     {"png"},
	"image/gif":                     {"gif"},
	"image/bmp":                     {"bmp"},
	"image/webp":                    {"webp"},
	"image/svg+xml":                 {"svg", "svgz"},
	"image/x-icon":                  {"ico"},
	"application/pdf":               {"pdf"},
	"application/zip":               {"zip", "docx", "xlsx", "pptx", "odt", "ods", "odp", "jar"},
	"application/x-gzip":            {"gz", "tgz"},
	"application/x-rar-compressed":  {"rar"},
	"application/vnd.ms-fontobject": {"eot"},
	"application/ogg":               {"ogg", "oga", "ogv"},
	"application/wasm":              {"wasm"},
	"application/json":              {"json"},
	"audio/mpeg":                    {"mp3"},
	"audio/wave":                    {"wav"},
	"audio/aiff":                    {"aif", "aiff"},
	"audio/midi":                    {"mid", "midi"},
	"video/mp4":                     {"mp4", "m4v"},
	"video/webm":                    {"webm"},
	"video/avi":                     {"avi"},
	"font/ttf":                      {"ttf"},
	"font/otf":                      {"otf"},
	"font/woff":                     {"woff"},
	"font/woff2":                    {"woff2"},
	"text/plain":                    {"txt", "text", "log", "csv", "md", "conf", "ini"},
	"text/html":                     {"html", "htm"},
	"text/xml":                      {"xml"},
	"text/csv":                      {"csv"},
}
