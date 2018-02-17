package webwire

import (
	"fmt"
	"time"
	"encoding/base64"
	cryptoRand "crypto/rand"
)

// generateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func generateRandomBytes(length uint32) (bytes []byte, err error) {
	bytes = make([]byte, length)
	_, err = cryptoRand.Read(bytes)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// GenerateSessionKey returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateSessionKey() string {
	bytes, err := generateRandomBytes(48)
	if err != nil {
		panic(fmt.Errorf("Could not generate a session key"))
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

type Session struct {
	Key string `json:"key"`
	OperatingSystem OperatingSystem `json:"os"`
	UserAgent string `json:"ua"`
	CreationDate time.Time `json:"crt"`
	Info interface {} `json:"inf"`
}

func NewSession(
	operatingSystem OperatingSystem,
	userAgent string,
	info interface {},
) Session {
	return Session {
		GenerateSessionKey(),
		operatingSystem,
		userAgent,
		time.Now(),
		info,
	}
}
