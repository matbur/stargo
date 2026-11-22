package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/matbur/stargo/internal/encoding"
)

const immutableCache = "public, max-age=31536000, immutable"

// Option configures a Handler.
type Option func(*config)

type config struct {
	prefix string
}

// Prefix sets the URL prefix used by URL, StableURL, and path matching.
// It must be a rooted path such as "/assets"; a trailing slash is ignored.
func Prefix(prefix string) Option {
	return func(c *config) {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		c.prefix = strings.TrimRight(prefix, "/")
	}
}

type asset struct {
	name        string
	hashedName  string
	contentType string
	content     []byte
	etag        string
	variants    map[string]variant
	modTime     time.Time
}

type variant struct {
	content []byte
	etag    string
}

// Handler serves an indexed set of static files.
type Handler struct {
	prefix string
	files  map[string]*asset
	hashed map[string]*asset
}

// FS indexes files from fsys and returns a handler.
func FS(fsys fs.FS, opts ...Option) (*Handler, error) {
	if fsys == nil {
		return nil, errors.New("assets: filesystem is nil")
	}
	return build(fsys, opts...)
}

// Dir indexes files under root and returns a handler. Symlinks inside root are
// not followed.
func Dir(root string, opts ...Option) (*Handler, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("assets: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("assets: %s is not a directory", root)
	}
	return build(os.DirFS(root), opts...)
}

func build(fsys fs.FS, opts ...Option) (*Handler, error) {
	cfg := config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	handler := &Handler{
		prefix: cfg.prefix,
		files:  make(map[string]*asset),
		hashed: make(map[string]*asset),
	}
	rawVariants := make(map[string]map[string][]byte)

	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if hiddenPath(name) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if encoding, base, ok := variantName(name); ok {
			if _, allowed := contentTypeOf(base); !allowed {
				return nil
			}
			if rawVariants[base] == nil {
				rawVariants[base] = make(map[string][]byte)
			}
			content, err := fs.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			rawVariants[base][encoding] = content
			return nil
		}

		contentType, allowed := contentTypeOf(name)
		if !allowed {
			return nil
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		file := &asset{
			name:        name,
			hashedName:  hashedName(name, sum),
			contentType: contentType,
			content:     content,
			etag:        quoteETag(hex.EncodeToString(sum[:])),
			modTime:     info.ModTime(),
		}
		handler.files[name] = file
		handler.hashed[file.hashedName] = file
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("assets: index: %w", err)
	}

	for name, encodings := range rawVariants {
		file := handler.files[name]
		if file == nil {
			continue
		}
		file.variants = make(map[string]variant, len(encodings))
		for encoding, content := range encodings {
			sum := sha256.Sum256(content)
			file.variants[encoding] = variant{
				content: content,
				etag:    quoteETag(hex.EncodeToString(sum[:])),
			}
		}
	}

	return handler, nil
}

// URL returns the prefixed content-hashed path for name. It panics when name
// was not indexed, because asset names are programmer-authored constants.
func (h *Handler) URL(name string) string {
	file := h.file(name)
	return h.path(file.hashedName)
}

// StableURL returns the prefixed plain path for name. It panics when name was
// not indexed.
func (h *Handler) StableURL(name string) string {
	file := h.file(name)
	return h.path(file.name)
}

func (h *Handler) file(name string) *asset {
	if !safePath(name) {
		panic(fmt.Sprintf("assets: unsafe asset name %q", name))
	}
	if file := h.files[name]; file != nil {
		return file
	}
	panic(fmt.Sprintf("assets: unknown asset %q", name))
}

func (h *Handler) path(name string) string {
	return h.prefix + "/" + name
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	name, ok := h.requestName(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	file := h.hashed[name]
	hashed := file != nil
	if file == nil {
		file = h.files[name]
	}
	if file == nil {
		http.NotFound(w, r)
		return
	}

	content := file.content
	etag := file.etag
	selected := ""
	if len(file.variants) > 0 {
		w.Header().Set("Vary", "Accept-Encoding")
		available := make([]string, 0, len(file.variants))
		for _, name := range []string{"br", "zstd", "gzip"} {
			if _, ok := file.variants[name]; ok {
				available = append(available, name)
			}
		}
		selected = encoding.Select(r.Header.Get("Accept-Encoding"), available...)
		if selected != "" {
			variant := file.variants[selected]
			content = variant.content
			etag = variant.etag
			w.Header().Set("Content-Encoding", selected)
		}
	}

	if hashed {
		w.Header().Set("Cache-Control", immutableCache)
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", file.contentType)
	http.ServeContent(w, r, file.name, file.modTime, bytes.NewReader(content))
}

func (h *Handler) requestName(requestPath string) (string, bool) {
	if h.prefix != "" {
		if requestPath == h.prefix {
			return "", false
		}
		if !strings.HasPrefix(requestPath, h.prefix+"/") {
			return "", false
		}
		requestPath = strings.TrimPrefix(requestPath, h.prefix)
	}
	requestPath = strings.TrimPrefix(requestPath, "/")
	if requestPath == "" || !safePath(requestPath) {
		return "", false
	}
	return requestPath, true
}

func safePath(name string) bool {
	if strings.ContainsAny(name, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return path.Clean(name) == name
}

func hiddenPath(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func hashedName(name string, sum [sha256.Size]byte) string {
	extension := path.Ext(name)
	base := strings.TrimSuffix(name, extension)
	return base + "." + hex.EncodeToString(sum[:])[:10] + extension
}

func quoteETag(value string) string {
	return `"` + value + `"`
}

func variantName(name string) (encoding, base string, ok bool) {
	for _, suffix := range []string{".br", ".zst", ".gz"} {
		if strings.HasSuffix(name, suffix) {
			return variantEncodings[suffix], strings.TrimSuffix(name, suffix), true
		}
	}
	return "", "", false
}

var variantEncodings = map[string]string{
	".br":  "br",
	".zst": "zstd",
	".gz":  "gzip",
}

func contentTypeOf(name string) (string, bool) {
	contentType, ok := contentTypes[strings.ToLower(path.Ext(name))]
	return contentType, ok
}

var contentTypes = map[string]string{
	".avif":  "image/avif",
	".css":   "text/css; charset=utf-8",
	".csv":   "text/csv; charset=utf-8",
	".gif":   "image/gif",
	".ico":   "image/x-icon",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".js":    "text/javascript; charset=utf-8",
	".json":  "application/json",
	".map":   "application/json",
	".mjs":   "text/javascript; charset=utf-8",
	".otf":   "font/otf",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".ttf":   "font/ttf",
	".txt":   "text/plain; charset=utf-8",
	".wasm":  "application/wasm",
	".webp":  "image/webp",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".xml":   "application/xml; charset=utf-8",
}
