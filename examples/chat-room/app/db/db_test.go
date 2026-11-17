package db_test

import (
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/db"
)

const schema = `
CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	body TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_body ON notes(body);
`

func TestOpenInMemoryAndApplySchemaTwice(t *testing.T) {
	t.Parallel()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.ApplySchema(schema); err != nil {
		t.Fatalf("first ApplySchema() error = %v", err)
	}
	if err := database.ApplySchema(schema); err != nil {
		t.Fatalf("second ApplySchema() error = %v", err)
	}

	if _, err := database.Exec("INSERT INTO notes (id, body) VALUES (?, ?)", "1", "hello"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	t.Parallel()

	database, err := db.Open("/does/not/exist/chat.db")
	if err == nil {
		_ = database.Close()
		t.Fatal("Open() did not fail for an unwritable path")
	}
}
