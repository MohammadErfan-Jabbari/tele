package appkey

import (
	"errors"
	"strconv"
	"strings"
)

const (
	EnvAPIID   = "TELEGRAM_API_ID"
	EnvAPIHash = "TELEGRAM_API_HASH"
)

var ErrInvalidEnvID = errors.New("TELEGRAM_API_ID must be a positive integer")

// FromEnv reads TELEGRAM_API_ID and TELEGRAM_API_HASH from getenv. Both must
// be set together; partial or invalid environment values return an error rather
// than falling back silently.
func FromEnv(getenv func(string) string) (Key, error) {
	rawID := strings.TrimSpace(getenv(EnvAPIID))
	hash := strings.TrimSpace(getenv(EnvAPIHash))
	if rawID == "" && hash == "" {
		return Key{}, nil
	}
	if rawID == "" || hash == "" {
		return Key{}, ErrHalfKey
	}
	id, err := strconv.Atoi(rawID)
	if err != nil || id <= 0 {
		return Key{}, ErrInvalidEnvID
	}
	return Key{ID: id, Hash: hash}, nil
}

// ResolveWithEnv gives an environment key precedence over config.yml,
// build-time ldflags, and the published key.
func ResolveWithEnv(env, user, injected Key) (Key, Source, error) {
	switch {
	case env.whole():
		return env, SourceUser, nil
	case !env.empty():
		return Key{}, "", ErrHalfKey
	default:
		return Resolve(user, injected)
	}
}
