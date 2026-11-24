// Command auth demonstrates password login, a server-side session, and a
// protected page.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/matbur/stargo"
	"github.com/matbur/stargo/auth"
	"github.com/matbur/stargo/forms"
	"github.com/matbur/stargo/route"
	"github.com/matbur/stargo/session"
)

const (
	demoUserID       = "demo"
	demoUsername     = "demo"
	fallbackPassword = "stargo-demo"
)

var (
	loginRoute = route.Get("/login")
	loginPost  = route.Post("/login")
	homeRoute  = route.Get("/")
	logoutPost = route.Post("/logout")
)

type application struct {
	passwordHash string
}

func newApp(password string, opts ...stargo.Option) (*stargo.App, error) {
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	manager, err := session.New(session.NewMemoryStore(), session.Insecure())
	if err != nil {
		return nil, err
	}

	app := stargo.New(opts...)
	app.Use(manager.Middleware())

	require := auth.RequireUser(auth.RedirectTo("/login"))
	app.Use(func(next http.Handler) http.Handler {
		protected := require(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" {
				next.ServeHTTP(w, r)
				return
			}
			protected.ServeHTTP(w, r)
		})
	})

	handlers := application{passwordHash: passwordHash}
	app.Handle(loginRoute, handlers.showLogin)
	app.Handle(loginPost, handlers.login)
	app.Handle(homeRoute, handlers.home)
	app.Handle(logoutPost, handlers.logout)
	return app, nil
}

func (a application) showLogin(c *stargo.Ctx) error {
	if _, ok := auth.UserFromRequest(c.R); ok {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	return c.HTML(loginPage(""))
}

func (a application) login(c *stargo.Ctx) error {
	form, err := forms.Read(c.R)
	if err != nil {
		return stargo.NewHTTPError(http.StatusBadRequest, "expected a form")
	}
	defer form.Close()

	if form.Get("username") != demoUsername {
		return c.HTML(loginPage("Invalid username or password."))
	}
	ok, err := auth.VerifyPassword(form.Get("password"), a.passwordHash)
	if err != nil {
		return err
	}
	if !ok {
		return c.HTML(loginPage("Invalid username or password."))
	}
	if err := auth.Login(session.FromRequest(c.R), auth.User{ID: demoUserID, Name: demoUsername}); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/")
}

func (a application) home(c *stargo.Ctx) error {
	if c.R.URL.Path != "/" {
		return stargo.NewHTTPError(http.StatusNotFound, "")
	}
	user, ok := auth.UserFromRequest(c.R)
	if !ok {
		return stargo.NewHTTPError(http.StatusUnauthorized, "")
	}
	return c.HTML(homePage(user))
}

func (a application) logout(c *stargo.Ctx) error {
	auth.Logout(session.FromRequest(c.R))
	return c.Redirect(http.StatusSeeOther, "/login")
}

func main() {
	password := os.Getenv("AUTH_PASSWORD")
	if password == "" {
		password = fallbackPassword
	}
	app, err := newApp(password)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = app.Close() }()

	if err := app.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
