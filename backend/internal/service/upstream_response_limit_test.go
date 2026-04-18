package service

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveUpstreamResponseReadLimit(t *testing.T) {
	t.Run("use default when config missing", func(t *testing.T) {
		require.Equal(t, defaultUpstreamResponseReadMaxBytes, resolveUpstreamResponseReadLimit(nil))
	})

	t.Run("use configured value", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Gateway.UpstreamResponseReadMaxBytes = 1234
		require.Equal(t, int64(1234), resolveUpstreamResponseReadLimit(cfg))
	})
}

func TestReadUpstreamResponseBodyLimited(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		body, err := readUpstreamResponseBodyLimited(bytes.NewReader([]byte("ok")), 2)
		require.NoError(t, err)
		require.Equal(t, []byte("ok"), body)
	})

	t.Run("exceeds limit", func(t *testing.T) {
		body, err := readUpstreamResponseBodyLimited(bytes.NewReader([]byte("toolong")), 3)
		require.Nil(t, body)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrUpstreamResponseBodyTooLarge))
	})
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestReadUpstreamResponseBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("too large triggers writer", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Gateway.UpstreamResponseReadMaxBytes = 3

		called := false
		body, err := ReadUpstreamResponseBody(bytes.NewReader([]byte("toolong")), cfg, nil, func(_ *gin.Context) {
			called = true
		})
		require.Nil(t, body)
		require.ErrorIs(t, err, ErrUpstreamResponseBodyTooLarge)
		require.True(t, called)
	})

	t.Run("io error does not trigger too large writer", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Gateway.UpstreamResponseReadMaxBytes = 3

		called := false
		body, err := ReadUpstreamResponseBody(errReader{}, cfg, nil, func(_ *gin.Context) {
			called = true
		})
		require.Nil(t, body)
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrUpstreamResponseBodyTooLarge))
		require.False(t, called)
	})
}
