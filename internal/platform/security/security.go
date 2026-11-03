package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// HashPassword genera un hash Argon2id de la contraseña.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		3,   // iterations
		64*1024,
		4,   // parallelism
		32,  // key length
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, 64*1024, 3, 4, b64Salt, b64Hash), nil
}

// ComparePassword compara una contraseña contra un hash Argon2id.
func ComparePassword(password, encodedHash string) (bool, error) {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	computed := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLen,
	)

	return subtle.ConstantTimeCompare(hash, computed) == 1, nil
}

type hashParams struct {
	memory   uint32
	iterations uint32
	parallelism uint8
	keyLen   uint32
}

func decodeHash(encodedHash string) (hashParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return hashParams{}, nil, nil, errors.New("formato de hash inválido")
	}

	var p hashParams
	var version int
	var m, t int
	var par int
	var kl int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil || version != argon2.Version {
		return hashParams{}, nil, nil, errors.New("versión de argon2 no soportada")
	}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &par)
	if err != nil {
		return hashParams{}, nil, nil, errors.New("parámetros de argon2 inválidos")
	}
	// parts[4] no trae key length en nuestro formato, usamos 32
	kl = 32

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return hashParams{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return hashParams{}, nil, nil, err
	}

	p.memory = uint32(m)
	p.iterations = uint32(t)
	p.parallelism = uint8(par)
	p.keyLen = uint32(kl)

	return p, salt, hash, nil
}

// GenerateToken genera un token aleatorio de 32 bytes en base64 URL.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// TokenExpiry devuelve el tiempo de expiración desde ahora.
func TokenExpiry(d time.Duration) time.Time {
	return time.Now().Add(d)
}

// HashToken hashea un token opaco (refresh token o API key) con SHA-256.
// Los tokens tienen alta entropía, por lo que SHA-256 es suficiente y permite
// búsquedas indexadas por hash.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CompareToken compara un token contra su hash SHA-256 en tiempo constante.
func CompareToken(token, expectedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashToken(token)), []byte(expectedHash)) == 1
}
