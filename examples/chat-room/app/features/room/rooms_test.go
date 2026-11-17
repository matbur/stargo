package room_test

import (
	"testing"

	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
	"github.com/matbur/stargo/examples/chat-room/app/features/room"
)

func testDB(t *testing.T) *chatdb.DB {
	t.Helper()

	database, err := chatdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.ApplySchema(room.Schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return database
}

func TestListRoomsOrderWithoutCase(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	for _, title := range []string{"zebra", "Apple", "mango"} {
		if _, err := room.AddRoom(database, title, title+" room"); err != nil {
			t.Fatalf("AddRoom(%q): %v", title, err)
		}
	}

	rooms, err := room.ListRooms(database)
	if err != nil {
		t.Fatalf("ListRooms() error = %v", err)
	}
	got := []string{rooms[0].Title, rooms[1].Title, rooms[2].Title}
	want := []string{"Apple", "mango", "zebra"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestListRoomsEmpty(t *testing.T) {
	t.Parallel()

	rooms, err := room.ListRooms(testDB(t))
	if err != nil {
		t.Fatalf("ListRooms() error = %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("rooms = %v, want none", rooms)
	}
}

func TestAddRoomSlugsAreUnique(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	titles := []string{"Design Chat", "design  chat", "Design Chat!"}
	want := []string{"design-chat", "design-chat-2", "design-chat-3"}

	for i, title := range titles {
		added, err := room.AddRoom(database, title, "talk")
		if err != nil {
			t.Fatalf("AddRoom(%q): %v", title, err)
		}
		if added.ID != want[i] {
			t.Fatalf("AddRoom(%q) id = %q, want %q", title, added.ID, want[i])
		}
	}
}

func TestAddRoomUnslugifiableTitle(t *testing.T) {
	t.Parallel()

	added, err := room.AddRoom(testDB(t), "🐾", "pets")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}
	if added.ID != "room" {
		t.Fatalf("id = %q, want room", added.ID)
	}
}

func TestGetRoom(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	added, err := room.AddRoom(database, "General", "Welcome")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	got, err := room.GetRoom(database, added.ID)
	if err != nil {
		t.Fatalf("GetRoom() error = %v", err)
	}
	if got == nil || *got != added {
		t.Fatalf("GetRoom() = %+v, want %+v", got, added)
	}

	missing, err := room.GetRoom(database, "nope")
	if err != nil {
		t.Fatalf("GetRoom(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetRoom(missing) = %+v, want nil", missing)
	}
}

func TestDeleteRoomCascades(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	added, err := room.AddRoom(database, "Temp", "Gone soon")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	if _, err := database.Exec(
		`INSERT INTO messages (id, room_id, user_id, username, color, text, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"m1", added.ID, "u1", "HappyFox", "#000", "hi", 1,
	); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO users (id, room_id, username, color, typing)
		 VALUES (?, ?, ?, ?, ?)`,
		"u1", added.ID, "HappyFox", "#000", 0,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	deleted, err := room.DeleteRoom(database, added.ID)
	if err != nil {
		t.Fatalf("DeleteRoom() error = %v", err)
	}
	if !deleted {
		t.Fatal("DeleteRoom() = false, want true")
	}

	for _, query := range []string{
		"SELECT COUNT(*) FROM rooms",
		"SELECT COUNT(*) FROM messages",
		"SELECT COUNT(*) FROM users",
	} {
		var count int
		if err := database.QueryRow(query).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Fatalf("%s = %d, want 0", query, count)
		}
	}
}

func TestDeleteMissingRoom(t *testing.T) {
	t.Parallel()

	deleted, err := room.DeleteRoom(testDB(t), "nope")
	if err != nil {
		t.Fatalf("DeleteRoom() error = %v", err)
	}
	if deleted {
		t.Fatal("DeleteRoom() = true, want false")
	}
}
