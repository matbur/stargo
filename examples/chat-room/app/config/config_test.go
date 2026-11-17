package config_test

import (
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/config"
)

func TestDefaultDBPath(t *testing.T) {
	t.Setenv("CHAT_DB_PATH", "")

	if got := config.FromEnv().DBPath; got != config.DefaultDBPath {
		t.Fatalf("DBPath = %q, want %q", got, config.DefaultDBPath)
	}
}

func TestOverrideDBPath(t *testing.T) {
	t.Setenv("CHAT_DB_PATH", "/tmp/chat.db")

	if got := config.FromEnv().DBPath; got != "/tmp/chat.db" {
		t.Fatalf("DBPath = %q, want /tmp/chat.db", got)
	}
}
