package uuid

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	googleuuid "github.com/google/uuid"
)

// New generates a new UUIDv7 based on the current timestamp.
// UUIDv7 is time-ordered and suitable for use as database primary keys.
//
// Format (RFC 4122):
// - 48 bits: Unix timestamp in milliseconds
// - 4 bits: version (0111 = 7)
// - 12 bits: random data
// - 2 bits: variant (10)
// - 62 bits: random data
func New() string {
	var uuid [16]byte

	// Get current timestamp in milliseconds
	now := time.Now()
	timestamp := uint64(now.UnixMilli())

	// Set timestamp (48 bits)
	binary.BigEndian.PutUint64(uuid[0:8], timestamp<<16)

	// Fill remaining bytes with random data
	if _, err := rand.Read(uuid[6:]); err != nil {
		// Fallback to standard UUIDv4 if random generation fails
		return googleuuid.New().String()
	}

	// Set version (4 bits) to 0111 (7)
	uuid[6] = (uuid[6] & 0x0f) | 0x70

	// Set variant (2 bits) to 10
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return formatUUID(uuid)
}

// formatUUID formats a 16-byte array as a UUID string
func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(uuid[0:4]),
		binary.BigEndian.Uint16(uuid[4:6]),
		binary.BigEndian.Uint16(uuid[6:8]),
		binary.BigEndian.Uint16(uuid[8:10]),
		uuid[10:16],
	)
}

// Parse validates and parses a UUID string
func Parse(s string) (string, error) {
	parsed, err := googleuuid.Parse(s)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

// IsValid checks if a string is a valid UUID
func IsValid(s string) bool {
	_, err := googleuuid.Parse(s)
	return err == nil
}
