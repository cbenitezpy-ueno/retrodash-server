package logging

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	t.Run("returns logger with JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		logger := Setup(&buf, "bridge")
		require.NotNil(t, logger)

		logger.Info("test message", "key", "value")

		var entry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)

		assert.Equal(t, "test message", entry["msg"])
		assert.Equal(t, "INFO", entry["level"])
		assert.Equal(t, "bridge", entry["component"])
		assert.Equal(t, "value", entry["key"])
		assert.Contains(t, entry, "time")
	})

	t.Run("defaults to stdout when writer is nil", func(t *testing.T) {
		logger := Setup(nil, "test")
		require.NotNil(t, logger)
	})

	t.Run("respects LOG_LEVEL env var", func(t *testing.T) {
		var buf bytes.Buffer
		t.Setenv("LOG_LEVEL", "WARN")
		logger := Setup(&buf, "bridge")
		require.NotNil(t, logger)

		// INFO should be suppressed at WARN level
		logger.Info("should not appear")
		assert.Empty(t, buf.String())

		// WARN should appear
		logger.Warn("visible warning")
		assert.Contains(t, buf.String(), "visible warning")
	})

	t.Run("bridges stdlib log to slog JSON", func(t *testing.T) {
		var buf bytes.Buffer
		t.Setenv("LOG_LEVEL", "INFO")
		Setup(&buf, "bridge")

		log.Println("stdlib message")

		var entry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &entry)
		require.NoError(t, err)

		assert.Equal(t, "stdlib message", entry["msg"])
		assert.Equal(t, "bridge", entry["component"])
	})

	t.Run("invalid LOG_LEVEL defaults to INFO", func(t *testing.T) {
		var buf bytes.Buffer
		t.Setenv("LOG_LEVEL", "INVALID")
		logger := Setup(&buf, "bridge")
		require.NotNil(t, logger)

		logger.Info("info message")
		assert.Contains(t, buf.String(), "info message")
	})
}

func TestSlogWriter(t *testing.T) {
	var buf bytes.Buffer
	t.Setenv("LOG_LEVEL", "INFO")
	logger := Setup(&buf, "test")

	w := &slogWriter{logger: logger}
	n, err := w.Write([]byte("hello\n"))
	require.NoError(t, err)
	assert.Equal(t, 6, n)

	var entry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, "hello", entry["msg"])
}
