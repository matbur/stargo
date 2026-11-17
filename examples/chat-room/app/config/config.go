// Package config reads the chat-room runtime configuration once at startup.
package config

import "os"

// Config holds the runtime configuration for the example.
type Config struct {
	// DBPath is the SQLite database path. The default is in-memory.
	DBPath string
}

// DefaultDBPath is used when CHAT_DB_PATH is unset or empty.
const DefaultDBPath = ":memory:"

// FromEnv returns the configuration described by the environment.
func FromEnv() Config {
	path := os.Getenv("CHAT_DB_PATH")
	if path == "" {
		path = DefaultDBPath
	}
	return Config{DBPath: path}
}
