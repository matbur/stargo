package room_test

import (
	"testing"

	"github.com/matbur/stargo/examples/chat-room/app/features/room"
)

func TestPresenceQueries(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	general, err := room.AddRoom(database, "General", "Welcome")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}
	design, err := room.AddRoom(database, "Design", "UI talk")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}

	users := []room.User{
		{ID: "b", RoomID: general.ID, Username: "BraveOwl", Color: "#111"},
		{ID: "a", RoomID: general.ID, Username: "HappyFox", Color: "#222", Typing: true},
		{ID: "c", RoomID: design.ID, Username: "MightyBear", Color: "#333"},
	}
	for _, user := range users {
		if err := room.AddUser(database, user); err != nil {
			t.Fatalf("AddUser(%s): %v", user.Username, err)
		}
	}

	listed, err := room.ListUsers(database, general.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(listed) != 2 || listed[0].Username != "BraveOwl" || listed[1].Username != "HappyFox" {
		t.Fatalf("users = %+v", listed)
	}
	if !listed[1].Typing {
		t.Fatal("typing flag did not round-trip")
	}

	counts, err := room.CountUsersByRoom(database)
	if err != nil {
		t.Fatalf("CountUsersByRoom() error = %v", err)
	}
	if counts[general.ID] != 2 || counts[design.ID] != 1 {
		t.Fatalf("counts = %v", counts)
	}

	exists, err := room.UserExists(database, "a", general.ID)
	if err != nil || !exists {
		t.Fatalf("UserExists(a) = %v, %v; want true, nil", exists, err)
	}
	exists, err = room.UserExists(database, "a", design.ID)
	if err != nil || exists {
		t.Fatalf("UserExists(a, design) = %v, %v; want false, nil", exists, err)
	}

	if err := room.RemoveUser(database, "a", general.ID); err != nil {
		t.Fatalf("RemoveUser() error = %v", err)
	}
	if err := room.RemoveUser(database, "missing", general.ID); err != nil {
		t.Fatalf("RemoveUser(missing) error = %v", err)
	}
	exists, err = room.UserExists(database, "a", general.ID)
	if err != nil || exists {
		t.Fatalf("UserExists after removal = %v, %v; want false, nil", exists, err)
	}
}

func TestSetUserTypingReportsChanges(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	added, err := room.AddRoom(database, "General", "Welcome")
	if err != nil {
		t.Fatalf("AddRoom() error = %v", err)
	}
	if err := room.AddUser(database, room.User{
		ID: "u1", RoomID: added.ID, Username: "HappyFox", Color: "#000",
	}); err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}

	changed, err := room.SetUserTyping(database, "u1", added.ID, true)
	if err != nil || !changed {
		t.Fatalf("SetUserTyping(true) = %v, %v; want true, nil", changed, err)
	}

	changed, err = room.SetUserTyping(database, "u1", added.ID, true)
	if err != nil || changed {
		t.Fatalf("SetUserTyping(true) again = %v, %v; want false, nil", changed, err)
	}

	users, err := room.ListUsers(database, added.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || !users[0].Typing {
		t.Fatalf("users = %+v", users)
	}

	changed, err = room.SetUserTyping(database, "missing", added.ID, false)
	if err != nil || changed {
		t.Fatalf("SetUserTyping(missing) = %v, %v; want false, nil", changed, err)
	}
}
