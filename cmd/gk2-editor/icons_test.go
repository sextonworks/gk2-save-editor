package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIconHandler(t *testing.T) {
	dir := t.TempDir()
	icon := filepath.Join(dir, "i_candle.png")
	require.NoError(t, os.WriteFile(icon, []byte("\x89PNG\r\n\x1a\n"), 0o600))
	h := iconHandler(func(name string) (string, error) {
		if name == "i_candle" {
			return icon, nil
		}
		return "", errors.New("no icon")
	})
	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/icons/i_candle.png", http.StatusOK},
		{http.MethodGet, "/icons/missing.png", http.StatusNotFound},
		{http.MethodGet, "/icons/i_candle", http.StatusNotFound},
		{http.MethodGet, "/other/i_candle.png", http.StatusNotFound},
		{http.MethodPost, "/icons/i_candle.png", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}
