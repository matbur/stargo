package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/session"
)

func testManager(t *testing.T) *session.Manager {
	t.Helper()
	manager, err := session.New(session.NewMemoryStore())
	if err != nil {
		t.Fatalf("session.New() error = %v", err)
	}
	return manager
}

func serve(handler http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func cookiesOf(recorder *httptest.ResponseRecorder) []*http.Cookie {
	return (&http.Response{Header: recorder.Header()}).Cookies()
}

func loginCookie(t *testing.T, manager *session.Manager, user User) *http.Cookie {
	t.Helper()
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Login(session.FromRequest(r), user); err != nil {
			t.Errorf("Login() error = %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := serve(handler, "/", nil)
	cookies := cookiesOf(recorder)
	if len(cookies) != 1 {
		t.Fatalf("login Set-Cookie count = %d", len(cookies))
	}
	return cookies[0]
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("hash = %q", hash)
	}
	second, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == second {
		t.Fatal("two hashes of the same password are equal")
	}

	ok, err := VerifyPassword("correct horse", hash)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = %v, %v", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("VerifyPassword(wrong) = %v, %v", ok, err)
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"plain",
		"$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$a2V5",
		"$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=0,t=3,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=0,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=0$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=2$not-base64$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$not-base64",
		"$argon2id$v=19$m=65536,t=3,p=2",
	}
	for _, encoded := range cases {
		if _, err := VerifyPassword("password", encoded); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("VerifyPassword(%q) error = %v", encoded, err)
		}
	}
}

func TestLoginRequiresSessionAndID(t *testing.T) {
	t.Parallel()

	if err := Login(nil, User{ID: "u1"}); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Login(nil) error = %v", err)
	}
	manager := testManager(t)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Login(session.FromRequest(r), User{}); !errors.Is(err, ErrNoUserID) {
			t.Errorf("Login(empty id) error = %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	serve(handler, "/", nil)
}

func TestLoginRotatesAndLoadsUser(t *testing.T) {
	t.Parallel()

	manager := testManager(t)
	firstHandler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("cart", "kept")
		w.WriteHeader(http.StatusNoContent)
	}))
	first := serve(firstHandler, "/", nil)
	firstCookie := cookiesOf(first)[0]

	loginHandler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Login(session.FromRequest(r), User{ID: "u1", Name: "Alice", Roles: []string{"admin"}}); err != nil {
			t.Errorf("Login() error = %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	login := serve(loginHandler, "/", firstCookie)
	cookies := cookiesOf(login)
	if len(cookies) != 1 {
		t.Fatalf("login Set-Cookie count = %d", len(cookies))
	}
	if cookies[0].Value == firstCookie.Value {
		t.Fatal("login did not rotate the session id")
	}

	probe := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromRequest(r)
		if !ok {
			t.Fatal("UserFromRequest() = false")
		}
		if user.ID != "u1" || user.Name != "Alice" || len(user.Roles) != 1 || user.Roles[0] != "admin" {
			t.Errorf("user = %+v", user)
		}
		if cart, ok := session.Value[string](session.FromRequest(r), "cart"); !ok || cart != "kept" {
			t.Errorf("cart = %q, %v", cart, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	serve(probe, "/", cookies[0])
}

func TestLogoutDestroysSession(t *testing.T) {
	t.Parallel()

	manager := testManager(t)
	cookie := loginCookie(t, manager, User{ID: "u1"})

	logoutHandler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logout(session.FromRequest(r))
		w.WriteHeader(http.StatusNoContent)
	}))
	logout := serve(logoutHandler, "/", cookie)
	cleared := cookiesOf(logout)
	if len(cleared) != 1 || cleared[0].Value != "" || cleared[0].MaxAge >= 0 {
		t.Fatalf("clearing cookies = %+v", cleared)
	}

	probe := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromRequest(r); ok {
			t.Error("logged-out request has a user")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	serve(probe, "/", cookie)
}

func TestMalformedStoredUserIsUnauthenticated(t *testing.T) {
	t.Parallel()

	manager := testManager(t)
	cookie := loginCookie(t, manager, User{ID: "u1"})

	corrupt := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set(userKey, "not json")
		w.WriteHeader(http.StatusNoContent)
	}))
	serve(corrupt, "/", cookie)

	probe := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromRequest(r); ok {
			t.Error("malformed user was accepted")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	serve(probe, "/", cookie)
}

func TestRequireUser(t *testing.T) {
	t.Parallel()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := serve(RequireUser()(next), "/", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("default status = %d", recorder.Code)
	}
	if called {
		t.Fatal("default gate called the handler")
	}

	custom := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	recorder = serve(RequireUser(Unauthorized(custom))(next), "/", nil)
	if recorder.Code != http.StatusTeapot {
		t.Fatalf("custom status = %d", recorder.Code)
	}

	recorder = serve(RequireUser(RedirectTo("/login"))(next), "/", nil)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/login" {
		t.Fatalf("redirect = %d %q", recorder.Code, recorder.Header().Get("Location"))
	}
	if called {
		t.Fatal("redirect gate called the handler")
	}

	manager := testManager(t)
	cookie := loginCookie(t, manager, User{ID: "u1"})
	handler := manager.Middleware()(RequireUser()(next))
	recorder = serve(handler, "/", cookie)
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("authenticated = %d, called = %v", recorder.Code, called)
	}

	if _, ok := UserFromSession(nil); ok {
		t.Fatal("UserFromSession(nil) accepted a user")
	}
	if _, ok := UserFromRequest(nil); ok {
		t.Fatal("UserFromRequest(nil) accepted a user")
	}
	Logout(nil)
}
