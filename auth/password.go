package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength    = 16
	keyLength     = 32
	memoryKiB     = 64 * 1024
	iterations    = 3
	parallelism   = 2
	hashAlgorithm = "argon2id"
)

// ErrInvalidHash is returned when a stored password hash cannot be parsed.
var ErrInvalidHash = errors.New("auth: invalid password hash")

type hashParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	keyLength   uint32
}

// HashPassword returns an argon2id PHC string for password.
//
// Every call uses a fresh random salt. The parameters travel with the hash, so
// hashes created with older settings keep verifying.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, parallelism, keyLength)
	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		hashAlgorithm,
		argon2.Version,
		memoryKiB,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded argon2id hash.
//
// A wrong password returns false without an error. A malformed hash returns
// ErrInvalidHash. The comparison runs in constant time.
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)
	return subtle.ConstantTimeCompare(computed, key) == 1, nil
}

func decodeHash(encoded string) (hashParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != hashAlgorithm {
		return hashParams{}, nil, nil, ErrInvalidHash
	}

	version, err := parseVersion(parts[2])
	if err != nil || version != argon2.Version {
		return hashParams{}, nil, nil, ErrInvalidHash
	}

	params, err := parseParams(parts[3])
	if err != nil {
		return hashParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	params.keyLength = uint32(len(key))
	return params, salt, key, nil
}

func parseVersion(value string) (int, error) {
	if !strings.HasPrefix(value, "v=") {
		return 0, ErrInvalidHash
	}
	version, err := strconv.ParseUint(strings.TrimPrefix(value, "v="), 10, 32)
	if err != nil {
		return 0, ErrInvalidHash
	}
	return int(version), nil
}

func parseParams(value string) (hashParams, error) {
	fields := strings.Split(value, ",")
	if len(fields) != 3 {
		return hashParams{}, ErrInvalidHash
	}
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		name, raw, found := strings.Cut(field, "=")
		if !found {
			return hashParams{}, ErrInvalidHash
		}
		values[name] = raw
	}

	memory, err := parseUint(values["m"], 32)
	if err != nil || memory == 0 {
		return hashParams{}, ErrInvalidHash
	}
	iterations, err := parseUint(values["t"], 32)
	if err != nil || iterations == 0 {
		return hashParams{}, ErrInvalidHash
	}
	lanes, err := parseUint(values["p"], 8)
	if err != nil || lanes == 0 {
		return hashParams{}, ErrInvalidHash
	}
	return hashParams{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(lanes),
	}, nil
}

func parseUint(value string, bits int) (uint64, error) {
	if value == "" {
		return 0, ErrInvalidHash
	}
	return strconv.ParseUint(value, 10, bits)
}
