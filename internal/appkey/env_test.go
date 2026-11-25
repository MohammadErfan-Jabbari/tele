package appkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Key
		err  error
	}{
		{name: "unset env returns empty key", env: map[string]string{}, want: Key{}},
		{
			name: "valid env returns complete key",
			env:  map[string]string{EnvAPIID: "12345678", EnvAPIHash: "0123456789abcdef0123456789abcdef"},
			want: Key{ID: 12345678, Hash: "0123456789abcdef0123456789abcdef"},
		},
		{
			name: "id without hash returns ErrHalfKey",
			env:  map[string]string{EnvAPIID: "12345"},
			err:  ErrHalfKey,
		},
		{
			name: "hash without id returns ErrHalfKey",
			env:  map[string]string{EnvAPIHash: "abcdef"},
			err:  ErrHalfKey,
		},
		{
			name: "non-numeric id returns ErrInvalidEnvID",
			env:  map[string]string{EnvAPIID: "notanumber", EnvAPIHash: "abcdef"},
			err:  ErrInvalidEnvID,
		},
		{
			name: "zero id returns ErrInvalidEnvID",
			env:  map[string]string{EnvAPIID: "0", EnvAPIHash: "abcdef"},
			err:  ErrInvalidEnvID,
		},
		{
			name: "negative id returns ErrInvalidEnvID",
			env:  map[string]string{EnvAPIID: "-1", EnvAPIHash: "abcdef"},
			err:  ErrInvalidEnvID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromEnv(func(k string) string { return tt.env[k] })
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveWithEnv(t *testing.T) {
	userKey := Key{ID: 1, Hash: "userhash"}
	envKey := Key{ID: 99, Hash: "envhash"}
	injectedKey := Key{ID: 2, Hash: "injectedhash"}

	t.Run("config user key outranks env and injected keys", func(t *testing.T) {
		got, src, err := ResolveWithEnv(userKey, envKey, injectedKey)
		require.NoError(t, err)
		assert.Equal(t, userKey, got)
		assert.Equal(t, SourceUser, src)
	})

	t.Run("half config key fails even when env key is set", func(t *testing.T) {
		_, _, err := ResolveWithEnv(Key{ID: 1}, envKey, injectedKey)
		require.ErrorIs(t, err, ErrHalfKey)
	})

	t.Run("env key outranks injected key when config is empty", func(t *testing.T) {
		got, src, err := ResolveWithEnv(Key{}, envKey, injectedKey)
		require.NoError(t, err)
		assert.Equal(t, envKey, got)
		assert.Equal(t, SourceEnv, src)
	})

	t.Run("half env key fails instead of falling back to injected", func(t *testing.T) {
		_, _, err := ResolveWithEnv(Key{}, Key{ID: 99}, injectedKey)
		require.ErrorIs(t, err, ErrHalfKey)
	})

	t.Run("negative env id is rejected by whole check", func(t *testing.T) {
		_, _, err := ResolveWithEnv(Key{}, Key{ID: -5, Hash: "h"}, injectedKey)
		require.ErrorIs(t, err, ErrHalfKey)
	})

	t.Run("empty config and env fall back to injected key", func(t *testing.T) {
		got, src, err := ResolveWithEnv(Key{}, Key{}, injectedKey)
		require.NoError(t, err)
		assert.Equal(t, injectedKey, got)
		assert.Equal(t, SourceInjected, src)
	})
}
