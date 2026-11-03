package ids

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

// UUIDv7 genera un UUID versión 7 ordenable por tiempo (RFC 9562).
// Devuelve 16 bytes listos para guardar en BINARY(16).
func UUIDv7() []byte {
	now := time.Now().UnixMilli()
	b := make([]byte, 16)

	// 1. Relleno aleatorio primero
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	// 2. Timestamp de 48 bits (big-endian) en los primeros 6 bytes
	binary.BigEndian.PutUint32(b[0:4], uint32(now>>16))
	binary.BigEndian.PutUint16(b[4:6], uint16(now&0xFFFF))

	// 3. Version 7 en los 4 bits superiores del byte 6
	b[6] = 0x70 | (b[6] & 0x0F)

	// 4. Variant RFC 4122 (10xx) en los 2 bits superiores del byte 8
	b[8] = 0x80 | (b[8] & 0x3F)

	return b
}

// UUIDv7String devuelve el UUIDv7 en formato hexadecimal (32 chars).
func UUIDv7String() string {
	return hex.EncodeToString(UUIDv7())
}

// StringToBin16 decodifica un string hex de 32 chars (o con formato UUID con guiones) en 16 bytes.
func StringToBin16(s string) []byte {
	if s == "" {
		return nil
	}
	cleaned := strings.ReplaceAll(s, "-", "")
	if len(cleaned) != 32 {
		return nil
	}
	b, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil
	}
	return b
}

// Bin16ToString convierte 16 bytes a string hex de 32 chars.
func Bin16ToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}
