package common

// Truncate limits value to limit runes, never splitting a UTF-8 sequence.
func Truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
