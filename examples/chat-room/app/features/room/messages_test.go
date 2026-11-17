package room_test

import (
	"fmt"
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/features/room"
)

func TestMessagesAreReturnedOldestFirst(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	added, err := room.AddRoom(database, "General", "Welcome")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	for i, id := range []string{"m2", "m1", "m3"} {
		if err := room.AddMessage(database, room.Message{
			ID:        id,
			RoomID:    added.ID,
			UserID:    "u1",
			Username:  "HappyFox",
			Color:     "#000",
			Text:      id,
			Timestamp: int64(i + 1),
		}); err != nil {
			t.Fatalf("AddMessage(%s): %v", id, err)
		}
	}

	messages, err := room.ListMessages(database, added.ID, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(messages))
	}
	for i, want := range []string{"m2", "m1", "m3"} {
		if messages[i].ID != want {
			t.Fatalf("message %d = %q, want %q", i, messages[i].ID, want)
		}
	}
}

func TestAddMessageTrimsHistory(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	added, err := room.AddRoom(database, "Busy", "Lots of talk")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	for i := 0; i < room.MessageHistoryLimit+5; i++ {
		if err := room.AddMessage(database, room.Message{
			ID:        fmt.Sprintf("m%03d", i),
			RoomID:    added.ID,
			UserID:    "u1",
			Username:  "HappyFox",
			Color:     "#000",
			Text:      fmt.Sprintf("message %d", i),
			Timestamp: int64(i),
		}); err != nil {
			t.Fatalf("AddMessage(%d): %v", i, err)
		}
	}

	messages, err := room.ListMessages(database, added.ID, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != room.MessageHistoryLimit {
		t.Fatalf("got %d messages, want %d", len(messages), room.MessageHistoryLimit)
	}
	if messages[0].ID != "m005" {
		t.Fatalf("oldest kept = %q, want m005", messages[0].ID)
	}
	if messages[len(messages)-1].ID != "m104" {
		t.Fatalf("newest kept = %q, want m104", messages[len(messages)-1].ID)
	}
}
