package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/matbur/stargo/session"
)

// ErrNoSession is returned by Login without a session.
var ErrNoSession = errors.New("auth: no session")

// ErrNoUserID is returned by Login when the user has no id.
var ErrNoUserID = errors.New("auth: user id is empty")

const userKey = "auth.user"

// User is the authenticated user stored in the session.
//
// Roles is optional and application-defined. Values must be JSON-friendly
// because the session stores the user as JSON.
type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles,omitempty"`
}

// Login stores user in the session and rotates the session id.
//
// Rotate on login prevents session fixation. Login does not verify a password;
// call VerifyPassword first. A missing session or user id returns an error.
func Login(s *session.Session, user User) error {
	if s == nil {
		return ErrNoSession
	}
	if user.ID == "" {
		return ErrNoUserID
	}
	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}
	if err := s.Rotate(); err != nil {
		return err
	}
	s.Set(userKey, string(payload))
	return nil
}

// Logout destroys the session. A nil session is ignored.
func Logout(s *session.Session) {
	if s != nil {
		s.Destroy()
	}
}

// UserFromSession returns the authenticated user from the session.
//
// A missing, malformed, or id-less value reports false, so the gate fails
// closed.
func UserFromSession(s *session.Session) (User, bool) {
	payload, ok := session.Value[string](s, userKey)
	if !ok || payload == "" {
		return User{}, false
	}
	var user User
	if err := json.Unmarshal([]byte(payload), &user); err != nil || user.ID == "" {
		return User{}, false
	}
	return user, true
}

// UserFromRequest returns the authenticated user attached to the request.
func UserFromRequest(request *http.Request) (User, bool) {
	if request == nil {
		return User{}, false
	}
	return UserFromSession(session.FromRequest(request))
}
