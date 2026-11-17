package room

// Prefix matches every event for one room.
func Prefix(roomID string) string {
	return "room." + roomID + ".*"
}

// PresenceSubject is published when a user joins or leaves a room.
func PresenceSubject(roomID string) string {
	return "room." + roomID + ".presence"
}

// MessageSubject is published when a message is stored.
func MessageSubject(roomID string) string {
	return "room." + roomID + ".message"
}

// TypingSubject is published when a typing flag changes.
func TypingSubject(roomID string) string {
	return "room." + roomID + ".typing"
}

// DeletedSubject is published when a room is removed.
func DeletedSubject(roomID string) string {
	return "room." + roomID + ".deleted"
}
