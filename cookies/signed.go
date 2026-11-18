package cookies

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	signedVersion  = "v1"
	minKeyBytes    = 32
	maxCookieBytes = 4096
	defaultMaxAge  = 14 * 24 * time.Hour
	keyLabel       = "stargo.cookie."
)

// SignedValue is a verified, unexpired payload read by a Signer.
type SignedValue struct {
	// Data is the payload that was signed.
	Data map[string]any
	// IssuedAt is when the value was signed.
	IssuedAt time.Time
	// ExpiresAt is when the value stops being accepted.
	ExpiresAt time.Time
}

// Age returns the time since the value was signed.
func (v SignedValue) Age() time.Duration {
	return time.Since(v.IssuedAt)
}

// Signer signs and verifies one cookie whose JSON payload the client can read
// but not change.
//
// The payload is not encrypted: store an id, never a secret. A Signer is
// immutable after NewSigner and safe for concurrent use.
type Signer struct {
	name     string
	keys     [][]byte
	maxAge   time.Duration
	path     string
	domain   string
	secure   bool
	httpOnly bool
	sameSite http.SameSite
}

// SignerOption configures a Signer.
type SignerOption func(*Signer)

// WithMaxAge sets how long a signed value is accepted. The default is 14 days.
func WithMaxAge(age time.Duration) SignerOption {
	return func(s *Signer) { s.maxAge = age }
}

// WithPath sets the cookie path. The default is "/".
func WithPath(path string) SignerOption {
	return func(s *Signer) { s.path = path }
}

// WithDomain sets the cookie domain. The default is no domain.
func WithDomain(domain string) SignerOption {
	return func(s *Signer) { s.domain = domain }
}

// WithSecure sets the Secure flag. It is true by default.
func WithSecure(secure bool) SignerOption {
	return func(s *Signer) { s.secure = secure }
}

// WithHTTPOnly sets the HttpOnly flag. It is true by default.
func WithHTTPOnly(httpOnly bool) SignerOption {
	return func(s *Signer) { s.httpOnly = httpOnly }
}

// WithSameSite sets the SameSite value. The default is SameSite=Lax.
func WithSameSite(sameSite http.SameSite) SignerOption {
	return func(s *Signer) { s.sameSite = sameSite }
}

// NewSigner returns a Signer for the cookie name.
//
// keys are at least 32 bytes each; the first key signs and every key verifies,
// so a new key can be put first and the old one kept for one max age. The
// signing key is derived from the configured key and the cookie name, so one
// secret can serve several cookies and a value cannot move to another name.
func NewSigner(name string, keys []string, opts ...SignerOption) (*Signer, error) {
	signer := &Signer{
		name:     name,
		maxAge:   defaultMaxAge,
		path:     "/",
		secure:   true,
		httpOnly: true,
		sameSite: http.SameSiteLaxMode,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(signer)
		}
	}

	if err := validateName(name); err != nil {
		return nil, err
	}
	if signer.maxAge <= 0 {
		return nil, fmt.Errorf("%w: signed cookie %q needs a positive max age", ErrInvalid, name)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: signed cookie %q needs at least one key", ErrInvalid, name)
	}
	derived := make([][]byte, 0, len(keys))
	for _, key := range keys {
		if len(key) < minKeyBytes {
			return nil, fmt.Errorf("%w: a key for signed cookie %q is shorter than %d bytes", ErrInvalid, name, minKeyBytes)
		}
		derived = append(derived, deriveKey([]byte(key), name))
	}
	secure := signer.secure || signer.sameSite == http.SameSiteNoneMode
	if err := checkPrefix(name, secure, signer.path, signer.domain); err != nil {
		return nil, err
	}

	signer.keys = derived
	return signer, nil
}

// Set signs data and writes the cookie with a fresh expiry.
//
// Data must be JSON-encodable. An oversized value returns ErrInvalid without
// writing a header.
func (s *Signer) Set(w http.ResponseWriter, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	now := time.Now()
	payload, err := json.Marshal(signedPayload{
		Data:    data,
		Issued:  now.Unix(),
		Expires: now.Add(s.maxAge).Unix(),
	})
	if err != nil {
		return fmt.Errorf("cookies: sign payload for %q: %w", s.name, err)
	}
	body := signedVersion + "." + encode(payload)
	value := body + "." + encode(sign(s.keys[0], body))

	cookie := s.cookie(value)
	if len(cookie.String()) > maxCookieBytes {
		return fmt.Errorf("%w: signed cookie %q exceeds %d bytes", ErrInvalid, s.name, maxCookieBytes)
	}
	return Set(w, cookie)
}

// Get returns the verified, unexpired payload, or false when the cookie is
// absent, changed, from another cookie name, or expired.
func (s *Signer) Get(request *http.Request) (SignedValue, bool) {
	raw := Read(request)[s.name]
	if raw == "" || !isASCII(raw) {
		return SignedValue{}, false
	}
	lastDot := strings.LastIndexByte(raw, '.')
	if lastDot < 0 {
		return SignedValue{}, false
	}
	body, encodedSignature := raw[:lastDot], raw[lastDot+1:]
	version, encoded, found := strings.Cut(body, ".")
	if !found || version != signedVersion || encoded == "" || encodedSignature == "" {
		return SignedValue{}, false
	}

	signature, err := decode(encodedSignature)
	if err != nil {
		return SignedValue{}, false
	}
	verified := false
	for _, key := range s.keys {
		if hmac.Equal(sign(key, body), signature) {
			verified = true
			break
		}
	}
	if !verified {
		return SignedValue{}, false
	}

	rawPayload, err := decode(encoded)
	if err != nil {
		return SignedValue{}, false
	}
	var payload signedPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return SignedValue{}, false
	}
	if payload.Data == nil || payload.Expires <= time.Now().Unix() {
		return SignedValue{}, false
	}
	return SignedValue{
		Data:      payload.Data,
		IssuedAt:  time.Unix(payload.Issued, 0),
		ExpiresAt: time.Unix(payload.Expires, 0),
	}, true
}

// Delete writes a clearing cookie with the signer's name, path, domain, and
// flags.
func (s *Signer) Delete(w http.ResponseWriter) error {
	return Delete(w, s.cookie(""))
}

type signedPayload struct {
	Data    map[string]any `json:"d"`
	Issued  int64          `json:"i"`
	Expires int64          `json:"e"`
}

func (s *Signer) cookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     s.name,
		Value:    value,
		Path:     s.path,
		Domain:   s.domain,
		MaxAge:   int(s.maxAge / time.Second),
		Secure:   s.secure,
		HttpOnly: s.httpOnly,
		SameSite: s.sameSite,
	}
}

func deriveKey(key []byte, name string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(keyLabel + name))
	return mac.Sum(nil)
}

func sign(key []byte, body string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

func encode(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decode(text string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(text)
}

func isASCII(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] >= 0x80 {
			return false
		}
	}
	return true
}
