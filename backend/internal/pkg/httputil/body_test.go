package httputil

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestReadRequestBodyWithPrealloc_DecodesCompressedBodies(t *testing.T) {
	t.Parallel()

	payload := `{"model":"claude-sonnet-4-5","input":"hello"}`
	testCases := []struct {
		name     string
		encoding string
		encode   func(t *testing.T, body string) []byte
	}{
		{
			name:     "gzip",
			encoding: "gzip",
			encode: func(t *testing.T, body string) []byte {
				t.Helper()
				var buf bytes.Buffer
				zw := gzip.NewWriter(&buf)
				_, err := io.WriteString(zw, body)
				require.NoError(t, err)
				require.NoError(t, zw.Close())
				return buf.Bytes()
			},
		},
		{
			name:     "deflate",
			encoding: "deflate",
			encode: func(t *testing.T, body string) []byte {
				t.Helper()
				var buf bytes.Buffer
				zw := zlib.NewWriter(&buf)
				_, err := io.WriteString(zw, body)
				require.NoError(t, err)
				require.NoError(t, zw.Close())
				return buf.Bytes()
			},
		},
		{
			name:     "zstd",
			encoding: "zstd",
			encode: func(t *testing.T, body string) []byte {
				t.Helper()
				var buf bytes.Buffer
				zw, err := zstd.NewWriter(&buf)
				require.NoError(t, err)
				_, err = io.WriteString(zw, body)
				require.NoError(t, err)
				require.NoError(t, zw.Close())
				return buf.Bytes()
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded := tc.encode(t, payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(encoded))
			req.Header.Set("Content-Encoding", tc.encoding)
			req.Header.Set("Content-Length", "999")
			req.ContentLength = int64(len(encoded))

			body, err := ReadRequestBodyWithPrealloc(req)
			require.NoError(t, err)
			require.Equal(t, payload, string(body))
			require.Empty(t, req.Header.Get("Content-Encoding"))
			require.Empty(t, req.Header.Get("Content-Length"))
			require.Equal(t, int64(len(payload)), req.ContentLength)
		})
	}
}

func TestReadRequestBodyWithPrealloc_RejectsOversizedDecompressedBody(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("a", maxDecompressedBodySize+1)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := io.WriteString(zw, payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")

	_, err = ReadRequestBodyWithPrealloc(req)
	require.Error(t, err)
	require.True(t, errors.Is(err, errDecompressedBodyTooLarge))
}

func TestReadRequestBodyWithPrealloc_RejectsUnsupportedContentEncoding(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("raw"))
	req.Header.Set("Content-Encoding", "br")

	_, err := ReadRequestBodyWithPrealloc(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), `unsupported Content-Encoding`)
}
