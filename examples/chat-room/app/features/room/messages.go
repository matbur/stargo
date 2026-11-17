package room

import chatdb "github.com/matbur/stargo/examples/chat-room/app/db"

// AddMessage stores a message and trims the room to the newest history window.
func AddMessage(database *chatdb.DB, message Message) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO messages
			(id, room_id, user_id, username, color, text, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		message.ID,
		message.RoomID,
		message.UserID,
		message.Username,
		message.Color,
		message.Text,
		message.Timestamp,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`DELETE FROM messages
		 WHERE room_id = ? AND id NOT IN (
			SELECT id FROM messages
			WHERE room_id = ?
			ORDER BY timestamp DESC
			LIMIT ?
		 )`,
		message.RoomID, message.RoomID, MessageHistoryLimit,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ListMessages returns a room's messages oldest first.
func ListMessages(database *chatdb.DB, roomID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = MessageHistoryLimit
	}

	rows, err := database.Query(
		`SELECT id, room_id, user_id, username, color, text, timestamp
		 FROM messages
		 WHERE room_id = ?
		 ORDER BY timestamp ASC, id ASC
		 LIMIT ?`,
		roomID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(
			&message.ID,
			&message.RoomID,
			&message.UserID,
			&message.Username,
			&message.Color,
			&message.Text,
			&message.Timestamp,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
