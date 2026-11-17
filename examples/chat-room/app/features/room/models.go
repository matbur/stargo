package room

// Room is one chat room. The id is a slug derived from the title.
type Room struct {
	ID          string
	Title       string
	Description string
}

// Message is one stored chat message. Timestamp is Unix milliseconds.
type Message struct {
	ID        string
	RoomID    string
	UserID    string
	Username  string
	Color     string
	Text      string
	Timestamp int64
}

// User is one visitor present in a room.
type User struct {
	ID       string
	RoomID   string
	Username string
	Color    string
	Typing   bool
}

// Schema creates the room, message, and presence tables. It is idempotent.
const Schema = `
CREATE TABLE IF NOT EXISTS rooms (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	room_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	username TEXT NOT NULL,
	color TEXT NOT NULL,
	text TEXT NOT NULL,
	timestamp INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id TEXT NOT NULL,
	room_id TEXT NOT NULL,
	username TEXT NOT NULL,
	color TEXT NOT NULL,
	typing INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (id, room_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_room_time
ON messages(room_id, timestamp);
`

// MessageHistoryLimit is the number of messages kept per room.
const MessageHistoryLimit = 100
