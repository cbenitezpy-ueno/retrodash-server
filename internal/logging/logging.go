package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
)

// Setup configures structured JSON logging using slog.
// It sets the default slog logger to a JSONHandler that outputs to the given writer.
// If writer is nil, os.Stdout is used.
// The log level is read from the LOG_LEVEL environment variable (DEBUG, INFO, WARN, ERROR).
// If LOG_LEVEL is not set or invalid, defaults to INFO.
// It also bridges the stdlib log package to slog so all log.Printf calls produce JSON.
func Setup(writer io.Writer, component string) *slog.Logger {
	if writer == nil {
		writer = os.Stdout
	}

	level := slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		_ = level.UnmarshalText([]byte(v))
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler).With("component", component)
	slog.SetDefault(logger)

	// Bridge stdlib log to slog so log.Printf produces structured JSON
	log.SetOutput(&slogWriter{logger: logger})
	log.SetFlags(0) // slog handles timestamp

	return logger
}

// slogWriter adapts slog.Logger to io.Writer for stdlib log bridging.
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	// Trim trailing newline that log.Printf adds
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	w.logger.Info(msg)
	return len(p), nil
}
