package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_FileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	closer, err := Setup(Config{Level: "info", FilePath: logPath})
	require.NoError(t, err)
	defer closer()

	// Write a log line with a "component" field.
	log.Info().Str("component", "server").Msg("test log entry")

	// Read the file and verify JSON structure.
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.NotEmpty(t, lines)

	var entry map[string]interface{}
	err = json.Unmarshal([]byte(lines[0]), &entry)
	require.NoError(t, err)

	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test log entry", entry["message"])
	assert.Equal(t, "server", entry["component"])
	assert.Contains(t, entry, "time")
}

func TestSetup_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "a", "b", "c", "test.log")

	closer, err := Setup(Config{Level: "debug", FilePath: nestedPath})
	require.NoError(t, err)
	defer closer()

	// Verify the directory was created.
	info, err := os.Stat(filepath.Dir(nestedPath))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify logging works.
	log.Debug().Msg("nested dir test")

	data, err := os.ReadFile(nestedPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "nested dir test")
}

func TestSetup_InvalidPath_FallsBackToStderr(t *testing.T) {
	// Use a path that cannot be created (on Linux, /proc is not writable).
	invalidPath := "/proc/self/invalid_dir/cannot_create/test.log"

	closer, err := Setup(Config{Level: "info", FilePath: invalidPath})
	// No error should be returned — falls back to stderr.
	require.NoError(t, err)
	defer closer()

	// Logging should still work (to stderr).
	log.Info().Msg("fallback to stderr works")
}

func TestSetup_EmptyPath_StderrOnly(t *testing.T) {
	closer, err := Setup(Config{Level: "warn", FilePath: ""})
	require.NoError(t, err)
	defer closer()

	// Verify the level was set correctly.
	assert.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())

	// Logging should work without panic.
	log.Warn().Msg("stderr only test")
}

func TestSetLevel(t *testing.T) {
	// Set up with a known state.
	closer, err := Setup(Config{Level: "info", FilePath: ""})
	require.NoError(t, err)
	defer closer()

	// Verify initial level.
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())

	// Change to debug.
	SetLevel("debug")
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())

	// Change to error.
	SetLevel("error")
	assert.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())

	// Change to warn.
	SetLevel("warn")
	assert.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())

	// Invalid level defaults to info.
	SetLevel("invalid_level")
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())

	// Empty string defaults to info.
	SetLevel("")
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}

func TestReopenFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "reopen.log")

	closer, err := Setup(Config{Level: "info", FilePath: logPath})
	require.NoError(t, err)
	defer closer()

	// Write to the original file.
	log.Info().Msg("before rotate")

	// Simulate logrotate: rename the old file.
	rotatedPath := logPath + ".1"
	err = os.Rename(logPath, rotatedPath)
	require.NoError(t, err)

	// Reopen — this should create a new file at the original path.
	err = ReopenFile()
	require.NoError(t, err)

	// Write to the new file.
	log.Info().Msg("after rotate")

	// Verify the new file contains the post-rotate entry.
	newData, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(newData), "after rotate")
	assert.NotContains(t, string(newData), "before rotate")

	// Verify the old file still has the pre-rotate entry.
	oldData, err := os.ReadFile(rotatedPath)
	require.NoError(t, err)
	assert.Contains(t, string(oldData), "before rotate")
}
