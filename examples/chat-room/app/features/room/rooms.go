package room

import (
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"

	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// ListRooms returns every room ordered by title without case sensitivity.
func ListRooms(database *chatdb.DB) ([]Room, error) {
	rows, err := database.Query(
		"SELECT id, title, description FROM rooms ORDER BY title COLLATE NOCASE",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var room Room
		if err := rows.Scan(&room.ID, &room.Title, &room.Description); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

// GetRoom returns one room, or nil when it does not exist.
func GetRoom(database *chatdb.DB, id string) (*Room, error) {
	var room Room
	err := database.QueryRow(
		"SELECT id, title, description FROM rooms WHERE id = ?", id,
	).Scan(&room.ID, &room.Title, &room.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// AddRoom inserts a room and derives a unique slug from the title.
func AddRoom(database *chatdb.DB, title, description string) (Room, error) {
	tx, err := database.Begin()
	if err != nil {
		return Room{}, err
	}
	defer func() { _ = tx.Rollback() }()

	id, err := uniqueRoomID(tx, slugify(title))
	if err != nil {
		return Room{}, err
	}

	room := Room{ID: id, Title: title, Description: description}
	if _, err := tx.Exec(
		"INSERT INTO rooms (id, title, description) VALUES (?, ?, ?)",
		room.ID, room.Title, room.Description,
	); err != nil {
		return Room{}, err
	}
	if err := tx.Commit(); err != nil {
		return Room{}, err
	}
	return room, nil
}

// DeleteRoom removes a room and every message and presence row scoped to it.
// It reports whether the room existed.
func DeleteRoom(database *chatdb.DB, id string) (bool, error) {
	tx, err := database.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	err = tx.QueryRow("SELECT 1 FROM rooms WHERE id = ?", id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, statement := range []string{
		"DELETE FROM messages WHERE room_id = ?",
		"DELETE FROM users WHERE room_id = ?",
		"DELETE FROM rooms WHERE id = ?",
	} {
		if _, err := tx.Exec(statement, id); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func slugify(title string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if slug == "" {
		return "room"
	}
	return slug
}

func uniqueRoomID(tx *sql.Tx, base string) (string, error) {
	candidate := base
	for suffix := 2; ; suffix++ {
		var exists int
		err := tx.QueryRow("SELECT 1 FROM rooms WHERE id = ?", candidate).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = base + "-" + strconv.Itoa(suffix)
	}
}
