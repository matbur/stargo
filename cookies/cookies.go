// Package cookies writes, deletes, and reads HTTP cookies.
//
// Set validates a cookie before writing it and returns ErrInvalid instead of
// letting the standard library silently drop or rewrite a value a browser would
// reject. It also enforces the __Host- and __Secure- prefix rules.
//
// Import it as cookies:
//
//	import "github.com/matbur/stargo/cookies"
//
//	cookies.Set(c.W, &http.Cookie{Name: "theme", Value: "dark", Path: "/"})
//	theme := cookies.Read(c.R)["theme"]
package cookies

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrInvalid is returned when a cookie would be rejected or altered by a
// browser.
var ErrInvalid = errors.New("cookies: invalid cookie")

// reserved names are cookie attributes, which may not be used as cookie names.
var reservedNames = map[string]bool{
	"comment":     true,
	"domain":      true,
	"expires":     true,
	"httponly":    true,
	"max-age":     true,
	"partitioned": true,
	"path":        true,
	"samesite":    true,
	"secure":      true,
	"version":     true,
}

// Set writes one Set-Cookie header for cookie.
//
// SameSite=None sets Secure, because browsers reject a None cookie without it;
// that changes the caller's struct. Names, values, paths, domains, and
// same-site values are validated first, and an invalid cookie returns
// ErrInvalid without writing a header.
func Set(w http.ResponseWriter, cookie *http.Cookie) error {
	if cookie == nil {
		return fmt.Errorf("%w: nil cookie", ErrInvalid)
	}
	if cookie.SameSite == http.SameSiteNoneMode {
		cookie.Secure = true
	}
	if err := validate(cookie); err != nil {
		return err
	}
	http.SetCookie(w, cookie)
	return nil
}

// Delete writes a Set-Cookie header that clears the cookie.
//
// Pass the same name, path, domain, and flags used when the cookie was set,
// because browsers match on them. The caller's struct is not changed.
func Delete(w http.ResponseWriter, cookie *http.Cookie) error {
	if cookie == nil {
		return fmt.Errorf("%w: nil cookie", ErrInvalid)
	}
	clearing := *cookie
	clearing.Value = ""
	clearing.MaxAge = -1
	clearing.Expires = time.Unix(1, 0).UTC()
	clearing.Raw = ""
	clearing.RawExpires = ""
	clearing.Unparsed = nil
	if clearing.SameSite == http.SameSiteNoneMode {
		clearing.Secure = true
	}
	if err := validate(&clearing); err != nil {
		return err
	}
	http.SetCookie(w, &clearing)
	return nil
}

// Read returns the cookies sent with a request.
//
// Every Cookie header line is parsed in order, so a later value for the same
// name wins over an earlier one. Malformed pairs are skipped, a value may
// contain '=', and a double-quoted value is unquoted when possible.
func Read(request *http.Request) map[string]string {
	values := map[string]string{}
	if request == nil {
		return values
	}
	for _, line := range request.Header.Values("Cookie") {
		for _, pair := range strings.Split(line, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			name, value, found := strings.Cut(pair, "=")
			name = strings.TrimSpace(name)
			if !found || name == "" {
				continue
			}
			values[name] = unquote(strings.TrimSpace(value))
		}
	}
	return values
}

func unquote(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return value
	}
	return unquoted
}

func validate(cookie *http.Cookie) error {
	if err := validateName(cookie.Name); err != nil {
		return err
	}
	if err := validateValue(cookie.Value); err != nil {
		return err
	}
	if strings.ContainsAny(cookie.Path, ";,") {
		return fmt.Errorf("%w: path %q contains ';' or ','", ErrInvalid, cookie.Path)
	}
	if strings.ContainsAny(cookie.Domain, ";,") {
		return fmt.Errorf("%w: domain %q contains ';' or ','", ErrInvalid, cookie.Domain)
	}
	switch cookie.SameSite {
	case 0, http.SameSiteDefaultMode, http.SameSiteLaxMode, http.SameSiteStrictMode, http.SameSiteNoneMode:
	default:
		return fmt.Errorf("%w: same-site value %d", ErrInvalid, cookie.SameSite)
	}
	return checkPrefix(cookie.Name, cookie.Secure, cookie.Path, cookie.Domain)
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalid)
	}
	if reservedNames[strings.ToLower(name)] {
		return fmt.Errorf("%w: name %q is a cookie attribute", ErrInvalid, name)
	}
	for index := 0; index < len(name); index++ {
		if !isTokenChar(name[index]) {
			return fmt.Errorf("%w: name %q is not a token", ErrInvalid, name)
		}
	}
	return nil
}

func validateValue(value string) error {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character < 0x20 || character > 0x7e {
			return fmt.Errorf("%w: value contains a non-printable character", ErrInvalid)
		}
		switch character {
		case ';', ',', '"', '\\':
			return fmt.Errorf("%w: value contains %q", ErrInvalid, character)
		}
	}
	return nil
}

func checkPrefix(name string, secure bool, path, domain string) error {
	lowered := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lowered, "__host-"):
		if !secure || path != "/" || domain != "" {
			return fmt.Errorf("%w: __Host- cookie %q needs Secure, Path=/, and no Domain", ErrInvalid, name)
		}
	case strings.HasPrefix(lowered, "__secure-"):
		if !secure {
			return fmt.Errorf("%w: __Secure- cookie %q needs Secure", ErrInvalid, name)
		}
	}
	return nil
}

func isTokenChar(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z',
		character >= 'A' && character <= 'Z',
		character >= '0' && character <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character))
}
