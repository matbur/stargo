package session_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/matbur/stargo/session"
)

func TestMemoryStoreSaveLoadDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := session.NewMemoryStore()
	now := time.Now()
	values := map[string]any{"user": "alice"}
	record := session.Record{Values: values, Created: now, Expires: now.Add(time.Hour)}

	if err := store.Save(ctx, "id-1", record); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	values["user"] = "changed"

	loaded, ok, err := store.Load(ctx, "id-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() reported no record")
	}
	if got := loaded.Values["user"]; got != "alice" {
		t.Fatalf("loaded user = %v, want alice", got)
	}
	if !loaded.Created.Equal(now) || !loaded.Expires.Equal(record.Expires) {
		t.Fatalf("timestamps = %v %v", loaded.Created, loaded.Expires)
	}
	loaded.Values["user"] = "mutated"
	again, _, _ := store.Load(ctx, "id-1")
	if got := again.Values["user"]; got != "alice" {
		t.Fatalf("store alias leaked, user = %v", got)
	}

	if err := store.Delete(ctx, "id-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok, _ := store.Load(ctx, "id-1"); ok {
		t.Fatal("Load() found a deleted record")
	}
	if store.Len() != 0 {
		t.Fatalf("Len() = %d", store.Len())
	}
}

func TestMemoryStoreExpiryAndSweep(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := session.NewMemoryStore()
	now := time.Now()
	expired := session.Record{Values: map[string]any{"k": "v"}, Created: now.Add(-time.Hour), Expires: now.Add(-time.Minute)}
	live := session.Record{Values: map[string]any{"k": "v"}, Created: now, Expires: now.Add(time.Hour)}
	if err := store.Save(ctx, "expired", expired); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, "live", live); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", store.Len())
	}

	if _, ok, err := store.Load(ctx, "expired"); err != nil || ok {
		t.Fatalf("expired Load() = %v, %v", ok, err)
	}
	if store.Sweep() != 0 {
		t.Fatal("Sweep() removed an already-loaded record")
	}

	if err := store.Save(ctx, "expired-2", expired); err != nil {
		t.Fatal(err)
	}
	if removed := store.Sweep(); removed != 1 {
		t.Fatalf("Sweep() = %d, want 1", removed)
	}
	if store.Len() != 1 {
		t.Fatalf("Len() after Sweep = %d", store.Len())
	}
	if _, ok, _ := store.Load(ctx, "live"); !ok {
		t.Fatal("Sweep removed a live record")
	}
}

func TestMemoryStoreContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := session.NewMemoryStore()
	if _, _, err := store.Load(ctx, "id"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error = %v", err)
	}
	if err := store.Save(ctx, "id", session.Record{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Delete(ctx, "id"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete() error = %v", err)
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func newManager(t *testing.T, store session.Store, opts ...session.Option) *session.Manager {
	t.Helper()
	manager, err := session.New(store, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return manager
}

func serve(handler http.Handler, cookie *http.Cookie) (*httptest.ResponseRecorder, *http.Cookie) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != nil {
		request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder, responseCookie(recorder)
}

func responseCookie(recorder *httptest.ResponseRecorder) *http.Cookie {
	response := &http.Response{Header: recorder.Header()}
	cookies := response.Cookies()
	if len(cookies) == 0 {
		return nil
	}
	return cookies[0]
}

func TestMiddlewareCreatesAndLoadsSession(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	clock := newFakeClock()
	manager := newManager(t, store, session.WithClock(clock.Now))
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		if s == nil {
			t.Error("FromRequest() = nil")
			return
		}
		if _, ok := session.Value[string](s, "user"); !ok {
			s.Set("user", "alice")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	first, cookie := serve(handler, nil)
	if first.Code != http.StatusNoContent {
		t.Fatalf("status = %d", first.Code)
	}
	if cookie == nil {
		t.Fatal("first response set no cookie")
	}
	if cookie.Name != "__Host-session" {
		t.Fatalf("cookie name = %q", cookie.Name)
	}
	if cookie.Value == "" {
		t.Fatal("cookie value is empty")
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("cookie attributes = %+v", cookie)
	}
	if store.Len() != 1 {
		t.Fatalf("store.Len() = %d", store.Len())
	}
	clock.Advance(10 * time.Minute)

	second, refreshed := serve(handler, cookie)
	if second.Code != http.StatusNoContent {
		t.Fatalf("second status = %d", second.Code)
	}
	if refreshed != nil {
		t.Fatalf("unexpected refreshed cookie = %+v", refreshed)
	}
	if store.Len() != 1 {
		t.Fatalf("store.Len() after reuse = %d", store.Len())
	}
	record, ok, err := store.Load(context.Background(), cookie.Value)
	if err != nil || !ok {
		t.Fatalf("Load() = %v, %v", ok, err)
	}
	if record.Values["user"] != "alice" {
		t.Fatalf("stored user = %v", record.Values["user"])
	}
	if record.Expires.After(clock.Now().Add(31 * time.Minute)) {
		t.Fatalf("expiry did not slide: %v", record.Expires)
	}
}

func TestMiddlewareUnusedSession(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	manager := newManager(t, store)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session.FromRequest(r) == nil {
			t.Error("FromRequest() = nil")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	_, cookie := serve(handler, nil)
	if cookie != nil {
		t.Fatalf("unused session set cookie = %+v", cookie)
	}
	if store.Len() != 0 {
		t.Fatalf("unused session stored %d records", store.Len())
	}
}

func TestSessionDataAccess(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	manager := newManager(t, store)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		s.Set("count", 2)
		s.Set("name", "alice")
		if value, ok := session.Value[int](s, "count"); !ok || value != 2 {
			t.Errorf("Value[int] = %v, %v", value, ok)
		}
		if _, ok := session.Value[string](s, "count"); ok {
			t.Error("Value[string] accepted an int")
		}
		if value, ok := s.Get("name"); !ok || value != "alice" {
			t.Errorf("Get() = %v, %v", value, ok)
		}
		data := s.Data()
		data["name"] = "changed"
		if value, _ := s.Get("name"); value != "alice" {
			t.Errorf("Data() exposed the map, name = %v", value)
		}
		s.Delete("name")
		if _, ok := s.Get("name"); ok {
			t.Error("Delete() left the key")
		}
		s.Clear()
		if len(s.Data()) != 0 {
			t.Errorf("Clear() left %d values", len(s.Data()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	if _, cookie := serve(handler, nil); cookie == nil {
		t.Fatal("modified session set no cookie")
	}
}

func TestSessionRotateAndDestroy(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	manager := newManager(t, store)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		switch r.URL.Path {
		case "/set":
			s.Set("user", "alice")
		case "/rotate":
			if err := s.Rotate(); err != nil {
				t.Errorf("Rotate() error = %v", err)
			}
		case "/destroy":
			s.Destroy()
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	router := http.NewServeMux()
	router.Handle("/set", handler)
	router.Handle("/rotate", handler)
	router.Handle("/destroy", handler)

	request := func(path string, cookie *http.Cookie) (*httptest.ResponseRecorder, *http.Cookie) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if cookie != nil {
			req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder, responseCookie(recorder)
	}

	_, first := request("/set", nil)
	if first == nil {
		t.Fatal("set produced no cookie")
	}
	_, rotatedCookie := request("/rotate", first)
	if rotatedCookie == nil || rotatedCookie.Value == first.Value {
		t.Fatalf("rotated cookie = %+v", rotatedCookie)
	}
	if _, ok, _ := store.Load(context.Background(), first.Value); ok {
		t.Fatal("old id still loads after rotation")
	}
	record, ok, _ := store.Load(context.Background(), rotatedCookie.Value)
	if !ok || record.Values["user"] != "alice" {
		t.Fatalf("rotated record = %+v, %v", record, ok)
	}

	_, cleared := request("/destroy", rotatedCookie)
	if cleared == nil || cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("cleared cookie = %+v", cleared)
	}
	if _, ok, _ := store.Load(context.Background(), rotatedCookie.Value); ok {
		t.Fatal("destroyed id still loads")
	}
}

func TestSessionIdleExpiration(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	clock := newFakeClock()
	manager := newManager(t, store,
		session.WithClock(clock.Now),
		session.WithIdleTimeout(30*time.Minute),
		session.WithAbsoluteTimeout(0),
	)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		if _, ok := session.Value[string](s, "user"); !ok {
			s.Set("user", "alice")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	_, cookie := serve(handler, nil)
	if cookie == nil {
		t.Fatal("no cookie")
	}
	clock.Advance(20 * time.Minute)
	_, slid := serve(handler, cookie)
	if slid != nil {
		t.Fatalf("unexpected cookie refresh = %+v", slid)
	}
	clock.Advance(20 * time.Minute)
	serve(handler, cookie)
	if _, ok, _ := store.Load(context.Background(), cookie.Value); !ok {
		t.Fatal("session did not slide past the original idle deadline")
	}

	clock.Advance(31 * time.Minute)
	serve(handler, cookie)
	if _, ok, _ := store.Load(context.Background(), cookie.Value); ok {
		t.Fatal("expired session still loads")
	}
	if store.Len() != 1 {
		t.Fatalf("store.Len() after expiry = %d", store.Len())
	}
}

func TestSessionAbsoluteExpiration(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	clock := newFakeClock()
	manager := newManager(t, store,
		session.WithClock(clock.Now),
		session.WithIdleTimeout(30*time.Minute),
		session.WithAbsoluteTimeout(time.Hour),
	)
	var sawUser bool
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		if _, ok := session.Value[string](s, "user"); ok {
			sawUser = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		sawUser = false
		s.Set("user", "alice")
		w.WriteHeader(http.StatusNoContent)
	}))

	_, cookie := serve(handler, nil)
	if cookie == nil {
		t.Fatal("no cookie")
	}
	clock.Advance(25 * time.Minute)
	serve(handler, cookie)
	if !sawUser {
		t.Fatal("session did not load before the absolute deadline")
	}
	clock.Advance(36 * time.Minute)
	serve(handler, cookie)
	if sawUser {
		t.Fatal("session lived past the absolute deadline")
	}
}

func TestSessionUnknownCookie(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	manager := newManager(t, store)
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session.FromRequest(r)
		if _, ok := s.Get("user"); ok {
			t.Error("unknown cookie produced data")
		}
		s.Set("user", "alice")
		w.WriteHeader(http.StatusNoContent)
	}))

	_, cookie := serve(handler, &http.Cookie{Name: manager.CookieName(), Value: "unknown"})
	if cookie == nil || cookie.Value == "unknown" {
		t.Fatalf("new cookie = %+v", cookie)
	}
	if store.Len() != 1 {
		t.Fatalf("store.Len() = %d", store.Len())
	}
}

func TestSessionLoadFailure(t *testing.T) {
	t.Parallel()

	store := &failingStore{loadErr: errors.New("load failed")}
	var reported error
	manager := newManager(t, store, session.WithErrorHandler(func(err error) { reported = err }))
	called := false
	handler := manager.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	recorder, _ := serve(handler, &http.Cookie{Name: manager.CookieName(), Value: "id"})
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if called {
		t.Fatal("handler ran after a load failure")
	}
	if reported == nil || !errors.Is(reported, store.loadErr) {
		t.Fatalf("reported error = %v", reported)
	}
}

func TestSessionSaveFailure(t *testing.T) {
	t.Parallel()

	store := &failingStore{saveErr: errors.New("save failed")}
	var reported error
	manager := newManager(t, store, session.WithErrorHandler(func(err error) { reported = err }))
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("user", "alice")
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder, _ := serve(handler, nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if reported == nil || !errors.Is(reported, store.saveErr) {
		t.Fatalf("reported error = %v", reported)
	}
}

func TestInsecureManager(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	manager := newManager(t, store, session.Insecure())
	if manager.CookieName() != "session" {
		t.Fatalf("CookieName() = %q", manager.CookieName())
	}
	handler := manager.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.FromRequest(r).Set("user", "alice")
		w.WriteHeader(http.StatusNoContent)
	}))
	_, cookie := serve(handler, nil)
	if cookie == nil || cookie.Name != "session" || cookie.Secure {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestManagerValidation(t *testing.T) {
	t.Parallel()

	store := session.NewMemoryStore()
	cases := map[string][]session.Option{
		"nil store":         nil,
		"zero idle":         {session.WithIdleTimeout(0)},
		"negative idle":     {session.WithIdleTimeout(-time.Second)},
		"negative absolute": {session.WithAbsoluteTimeout(-time.Second)},
		"host prefix":       {session.WithCookieName("__Host-session"), session.WithSecure(false)},
		"empty name":        {session.WithCookieName("")},
	}
	for name, opts := range cases {
		var err error
		if name == "nil store" {
			_, err = session.New(nil, opts...)
		} else {
			_, err = session.New(store, opts...)
		}
		if err == nil {
			t.Errorf("%s: New() accepted invalid options", name)
		}
	}
}

func TestSessionWithoutMiddleware(t *testing.T) {
	t.Parallel()

	if session.FromContext(context.Background()) != nil {
		t.Fatal("FromContext() returned a session")
	}
	if session.FromRequest(nil) != nil {
		t.Fatal("FromRequest(nil) returned a session")
	}
	var nilSession *session.Session
	if _, ok := session.Value[string](nilSession, "user"); ok {
		t.Fatal("Value() on a nil session reported a value")
	}
}

type failingStore struct {
	loadErr   error
	saveErr   error
	deleteErr error
}

func (s *failingStore) Load(context.Context, string) (session.Record, bool, error) {
	if s.loadErr != nil {
		return session.Record{}, false, s.loadErr
	}
	return session.Record{}, false, nil
}

func (s *failingStore) Save(context.Context, string, session.Record) error {
	return s.saveErr
}

func (s *failingStore) Delete(context.Context, string) error {
	return s.deleteErr
}
