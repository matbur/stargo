package room

import (
	"database/sql"
	"errors"

	chatdb "github.com/matbur/stargo/examples/chat-room/app/db"
)

// AddUser inserts or replaces one presence row.
func AddUser(database *chatdb.DB, user User) error {
	_, err := database.Exec(
		`INSERT OR REPLACE INTO users (id, room_id, username, color, typing)
		 VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.RoomID, user.Username, user.Color, boolInt(user.Typing),
	)
	return err
}

// RemoveUser removes one presence row. Removing a missing row is not an error.
func RemoveUser(database *chatdb.DB, userID, roomID string) error {
	_, err := database.Exec(
		"DELETE FROM users WHERE id = ? AND room_id = ?", userID, roomID,
	)
	return err
}

// ListUsers returns every user present in a room.
func ListUsers(database *chatdb.DB, roomID string) ([]User, error) {
	rows, err := database.Query(
		"SELECT id, room_id, username, color, typing FROM users WHERE room_id = ? ORDER BY username",
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var (
			user   User
			typing int
		)
		if err := rows.Scan(&user.ID, &user.RoomID, &user.Username, &user.Color, &typing); err != nil {
			return nil, err
		}
		user.Typing = typing != 0
		users = append(users, user)
	}
	return users, rows.Err()
}

// CountUsersByRoom returns online counts for every room.
func CountUsersByRoom(database *chatdb.DB) (map[string]int, error) {
	rows, err := database.Query(
		"SELECT room_id, COUNT(*) AS n FROM users GROUP BY room_id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var (
			roomID string
			count  int
		)
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, err
		}
		counts[roomID] = count
	}
	return counts, rows.Err()
}

// UserExists reports whether a user is present in a room.
func UserExists(database *chatdb.DB, userID, roomID string) (bool, error) {
	var exists int
	err := database.QueryRow(
		"SELECT 1 FROM users WHERE id = ? AND room_id = ?", userID, roomID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// SetUserTyping updates the typing flag and reports whether it changed.
func SetUserTyping(database *chatdb.DB, userID, roomID string, typing bool) (bool, error) {
	tx, err := database.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var current int
	err = tx.QueryRow(
		"SELECT typing FROM users WHERE id = ? AND room_id = ?", userID, roomID,
	).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if (current != 0) == typing {
		return false, nil
	}

	if _, err := tx.Exec(
		"UPDATE users SET typing = ? WHERE id = ? AND room_id = ?",
		boolInt(typing), userID, roomID,
	); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
