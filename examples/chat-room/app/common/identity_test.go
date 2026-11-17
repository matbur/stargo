package common_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/common"
)

func TestMintFormat(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		identity := common.Mint()
		if len(identity.UserID) != 8 {
			t.Fatalf("UserID = %q, want 8 characters", identity.UserID)
		}
		if identity.Username == "" || identity.Color == "" {
			t.Fatalf("incomplete identity: %+v", identity)
		}
		if !strings.HasPrefix(identity.Color, "#") {
			t.Fatalf("Color = %q, want a hex color", identity.Color)
		}
		seen[identity.UserID] = true
	}
	if len(seen) != 20 {
		t.Fatalf("minted %d distinct ids, want 20", len(seen))
	}
}

func TestForPagePrefersCompleteQuery(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet,
		"/?user_id=abc12345&username=HappyFox&color=%23e74c3c", nil)
	got := common.ForPage(request)
	want := common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#e74c3c"}
	if got != want {
		t.Fatalf("ForPage() = %+v, want %+v", got, want)
	}
}

func TestForPageMintsWhenQueryIncomplete(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/?user_id=abc12345", nil)
	got := common.ForPage(request)
	if got.UserID == "abc12345" || len(got.UserID) != 8 {
		t.Fatalf("ForPage() = %+v, want a fresh identity", got)
	}
}

func TestFromSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		want    *common.Identity
		wantErr bool
	}{
		{
			name:   "post body",
			method: http.MethodPost,
			target: "/",
			body:   `{"user_id":"abc12345","username":"HappyFox","color":"#000"}`,
			want:   &common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#000"},
		},
		{
			name:   "get query",
			method: http.MethodGet,
			target: "/?datastar=%7B%22user_id%22%3A%22abc12345%22%2C%22username%22%3A%22HappyFox%22%2C%22color%22%3A%22%23000%22%7D",
			want:   &common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#000"},
		},
		{
			name:   "missing color",
			method: http.MethodPost,
			target: "/",
			body:   `{"user_id":"abc12345","username":"HappyFox"}`,
			want:   nil,
		},
		{
			name:    "malformed",
			method:  http.MethodPost,
			target:  "/",
			body:    `{"user_id":`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			got, err := common.FromSignals(request)
			if test.wantErr {
				if err == nil {
					t.Fatal("FromSignals() did not fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("FromSignals() error = %v", err)
			}
			if test.want == nil {
				if got != nil {
					t.Fatalf("FromSignals() = %+v, want nil", got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("FromSignals() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestSessionInitScript(t *testing.T) {
	t.Parallel()

	script := common.SessionInitScript()
	for _, want := range []string{
		`"chat_visitor"`,
		"sessionStorage",
		"$user_id",
		"$username",
		"$color",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %s", want, script)
		}
	}
}

func TestSignalsShape(t *testing.T) {
	t.Parallel()

	identity := common.Identity{UserID: "abc12345", Username: "HappyFox", Color: "#000"}
	got := identity.Signals()
	if got["user_id"] != "abc12345" || got["username"] != "HappyFox" || got["color"] != "#000" {
		t.Fatalf("Signals() = %v", got)
	}
}
