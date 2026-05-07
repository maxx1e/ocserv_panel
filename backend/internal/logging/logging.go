package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logging configuration.
type Config struct {
	Level    string // debug, info, warn, error
	FilePath string // path to log file (empty = stderr only)
}

var (
	mu       sync.Mutex
	filePath string
	logFile  *os.File
)

// Setup initializes the global zerolog logger.
// Returns a closer function for the log file.
func Setup(cfg Config) (closer func(), err error) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.MessageFieldName = "message"

	mu.Lock()
	defer mu.Unlock()

	setLevelLocked(cfg.Level)

	if cfg.FilePath == "" {
		// Stderr-only with ConsoleWriter.
		cw := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
		log.Logger = zerolog.New(cw).With().Timestamp().Logger()
		return func() {}, nil
	}

	// Create parent directory.
	dir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		// Fall back to stderr-only without returning an error.
		cw := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
		log.Logger = zerolog.New(cw).With().Timestamp().Logger()
		log.Warn().Str("path", cfg.FilePath).Err(err).Msg("failed to create log directory, falling back to stderr")
		return func() {}, nil
	}

	f, openErr := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if openErr != nil {
		// Fall back to stderr-only without returning an error.
		cw := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
		log.Logger = zerolog.New(cw).With().Timestamp().Logger()
		log.Warn().Str("path", cfg.FilePath).Err(openErr).Msg("failed to open log file, falling back to stderr")
		return func() {}, nil
	}

	filePath = cfg.FilePath
	logFile = f

	// Multi-writer: JSON to file, ConsoleWriter to stderr.
	cw := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
	multi := io.MultiWriter(f, cw)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	closer = func() {
		mu.Lock()
		defer mu.Unlock()
		if logFile != nil {
			logFile.Close()
			logFile = nil
		}
	}
	return closer, nil
}

// ReopenFile closes and reopens the log file (for logrotate support via SIGHUP).
func ReopenFile() error {
	mu.Lock()
	defer mu.Unlock()

	if filePath == "" || logFile == nil {
		return fmt.Errorf("no log file configured")
	}

	// Close old file.
	if err := logFile.Close(); err != nil {
		// Best effort — continue with reopen.
		_ = err
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("reopen log file %q: %w", filePath, err)
	}

	logFile = f

	// Rebuild multi-writer.
	cw := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02T15:04:05.000Z07:00"}
	multi := io.MultiWriter(f, cw)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	return nil
}

// SetLevel changes the global log level at runtime.
func SetLevel(level string) {
	mu.Lock()
	defer mu.Unlock()
	setLevelLocked(level)
}

func setLevelLocked(level string) {
	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn", "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
