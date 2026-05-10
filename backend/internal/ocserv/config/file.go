package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Sentinel errors for ConfigFileManager operations.
var (
	// ErrConfigLocked is returned when the config file lock cannot be acquired
	// because another operation already holds it.
	ErrConfigLocked = errors.New("config file is locked by another operation")

	// ErrBackupNotFound is returned when the requested backup number does not exist.
	ErrBackupNotFound = errors.New("config backup not found")

	// ErrConfigTooLarge is returned when content exceeds the 64 KB size limit.
	ErrConfigTooLarge = errors.New("config content exceeds 64KB limit")
)

const configMaxBytes = 64 * 1024 // 64 KB

// ConfigMetadata holds filesystem metadata for the config file.
type ConfigMetadata struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"lastModified"`
	Size         int64     `json:"size"`
}

// BackupInfo describes a single numbered backup of the config file.
type BackupInfo struct {
	Number    int       `json:"number"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
}

// ConfigFileManager handles safe I/O for an ocserv configuration file.
// It provides atomic writes, numbered backup rotation, and OS-level locking
// so that concurrent writes never corrupt the config.
//
// All operations are safe to call concurrently: the OS flock on the .lock
// sidecar file serialises writers at the process and kernel level.
type ConfigFileManager struct {
	path       string
	maxBackups int
}

// NewConfigFileManager returns a ConfigFileManager for the given path.
// maxBackups controls how many numbered backups (.1 through .maxBackups)
// are retained. The oldest backup is deleted when rotation would exceed the limit.
func NewConfigFileManager(path string, maxBackups int) *ConfigFileManager {
	return &ConfigFileManager{path: path, maxBackups: maxBackups}
}

// Read returns the current content of the config file together with its
// filesystem metadata. Returns an error if the file cannot be read.
func (m *ConfigFileManager) Read() (string, ConfigMetadata, error) {
	info, err := os.Stat(m.path)
	if err != nil {
		return "", ConfigMetadata{}, fmt.Errorf("stat config %s: %w", m.path, err)
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		return "", ConfigMetadata{}, fmt.Errorf("read config %s: %w", m.path, err)
	}

	meta := ConfigMetadata{
		Path:         m.path,
		LastModified: info.ModTime(),
		Size:         info.Size(),
	}
	return string(data), meta, nil
}

// Write saves new content to the config file.
// Before writing it:
//  1. Rejects content larger than 64 KB.
//  2. Acquires an exclusive OS flock on the .lock sidecar file.
//  3. Rotates existing numbered backups (.N → .N+1, deletes .maxBackups if present).
//  4. Copies the current live file to .1.
//  5. Writes new content atomically (temp file + rename).
//
// The lock is released when Write returns.
func (m *ConfigFileManager) Write(content string) error {
	if len(content) > configMaxBytes {
		return ErrConfigTooLarge
	}

	unlock, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()

	// Rotate existing backups before overwriting: .N → .N+1.
	if err := m.rotateBackups(); err != nil {
		return fmt.Errorf("rotate backups: %w", err)
	}

	// Copy the current live file to .1.
	if err := m.copyToBackup(1); err != nil {
		return fmt.Errorf("copy to backup .1: %w", err)
	}

	// Write atomically.
	return m.atomicWrite(content)
}

// ListBackups returns metadata for all existing numbered backup files,
// ordered ascending by backup number. Returns an empty slice when no backups
// exist yet.
func (m *ConfigFileManager) ListBackups() ([]BackupInfo, error) {
	var backups []BackupInfo
	for n := 1; n <= m.maxBackups; n++ {
		info, err := os.Stat(m.backupPath(n))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat backup .%d: %w", n, err)
		}
		backups = append(backups, BackupInfo{
			Number:    n,
			Timestamp: info.ModTime(),
			Size:      info.Size(),
		})
	}
	return backups, nil
}

// GetBackup returns the content of backup number n.
// Returns ErrBackupNotFound when that backup does not exist.
func (m *ConfigFileManager) GetBackup(n int) (string, error) {
	path := m.backupPath(n)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", ErrBackupNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read backup .%d: %w", n, err)
	}
	return string(data), nil
}

// Rollback replaces the live config with the content of backup number n.
// Before restoring it:
//  1. Acquires the flock.
//  2. Rotates existing backups so the current live file is preserved as .1
//     (making the rollback itself reversible).
//  3. Reads the target backup (now at position n+1 after the rotation).
//  4. Writes it atomically to the live path.
//
// Returns ErrBackupNotFound when the requested backup does not exist.
func (m *ConfigFileManager) Rollback(n int) error {
	unlock, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()

	// Verify the target backup exists before mutating anything.
	if _, err := os.Stat(m.backupPath(n)); os.IsNotExist(err) {
		return ErrBackupNotFound
	}

	// Rotate first so current live becomes .1 and target backup shifts to n+1.
	if err := m.rotateBackups(); err != nil {
		return fmt.Errorf("rotate backups before rollback: %w", err)
	}

	if err := m.copyToBackup(1); err != nil {
		return fmt.Errorf("preserve current before rollback: %w", err)
	}

	// After rotation the target is at n+1.
	targetPath := m.backupPath(n + 1)
	data, err := os.ReadFile(targetPath)
	if os.IsNotExist(err) {
		return ErrBackupNotFound
	}
	if err != nil {
		return fmt.Errorf("read rollback source .%d: %w", n+1, err)
	}

	return m.atomicWrite(string(data))
}

// DiffWithContent reads the current config file and returns a unified diff
// between the current content (as "old") and the proposed content (as "new").
// Returns an empty string when the two are identical.
// Returns an error when the current file cannot be read.
func (m *ConfigFileManager) DiffWithContent(proposed string) (string, error) {
	current, _, err := m.Read()
	if err != nil {
		return "", fmt.Errorf("read current config for diff: %w", err)
	}
	return unifiedDiff(
		strings.Split(current, "\n"),
		strings.Split(proposed, "\n"),
		m.path,
		m.path+" (proposed)",
	), nil
}

// DiffWithBackup reads backup number n and the current live config, then
// returns a unified diff with the backup as "old" and the current file as
// "new". Returns ErrBackupNotFound when that backup does not exist.
func (m *ConfigFileManager) DiffWithBackup(n int) (string, error) {
	backup, err := m.GetBackup(n)
	if err != nil {
		return "", err
	}
	current, _, err := m.Read()
	if err != nil {
		return "", fmt.Errorf("read current config for diff: %w", err)
	}
	return unifiedDiff(
		strings.Split(backup, "\n"),
		strings.Split(current, "\n"),
		fmt.Sprintf("%s.%d", m.path, n),
		m.path,
	), nil
}

// unifiedDiff produces a simple unified-style diff between lines a (old) and
// b (new). It uses an LCS to identify unchanged lines, emitting "-" prefixed
// lines for removals and "+" prefixed lines for additions.
//
// Returns an empty string when a and b are identical.
func unifiedDiff(a, b []string, nameA, nameB string) string {
	// Build the LCS table.
	la, lb := len(a), len(b)
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to build the diff hunks.
	type op struct {
		kind byte // ' ', '-', '+'
		line string
	}
	var ops []op
	i, j := la, lb
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			ops = append(ops, op{' ', a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, op{'+', b[j-1]})
			j--
		default:
			ops = append(ops, op{'-', a[i-1]})
			i--
		}
	}

	// Reverse (backtracking produced ops in reverse order).
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}

	// Check whether anything actually changed.
	hasDiff := false
	for _, o := range ops {
		if o.kind != ' ' {
			hasDiff = true
			break
		}
	}
	if !hasDiff {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- ")
	sb.WriteString(nameA)
	sb.WriteByte('\n')
	sb.WriteString("+++ ")
	sb.WriteString(nameB)
	sb.WriteByte('\n')
	for _, o := range ops {
		sb.WriteByte(o.kind)
		sb.WriteString(o.line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// --- internal helpers ---

// backupPath returns the filesystem path for backup number n.
func (m *ConfigFileManager) backupPath(n int) string {
	return fmt.Sprintf("%s.%d", m.path, n)
}

// lockPath returns the path of the advisory lock sidecar file.
func (m *ConfigFileManager) lockPath() string {
	return m.path + ".lock"
}

// acquireLock opens (or creates) the .lock sidecar file and acquires an
// exclusive non-blocking flock. Returns a release function and an error.
// Returns ErrConfigLocked when another process or goroutine holds the lock.
func (m *ConfigFileManager) acquireLock() (func(), error) {
	f, err := os.OpenFile(m.lockPath(), os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", m.lockPath(), err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrConfigLocked
		}
		return nil, fmt.Errorf("flock %s: %w", m.lockPath(), err)
	}

	release := func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}

// rotateBackups shifts backup N → N+1 from maxBackups-1 down to 1,
// deleting .maxBackups first to make room. Backup .1 is freed for the
// caller to fill with the current live file.
func (m *ConfigFileManager) rotateBackups() error {
	// Delete the oldest backup to make room.
	oldest := m.backupPath(m.maxBackups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove oldest backup %s: %w", oldest, err)
	}

	// Shift N → N+1 from the second-oldest down to .1.
	for i := m.maxBackups - 1; i >= 1; i-- {
		src := m.backupPath(i)
		dst := m.backupPath(i + 1)
		if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rename backup .%d to .%d: %w", i, i+1, err)
		}
	}

	return nil
}

// copyToBackup copies the current live config to backup number n.
// Does nothing if the live config does not exist yet (first write).
func (m *ConfigFileManager) copyToBackup(n int) error {
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return nil // nothing to back up on first write
	}
	if err != nil {
		return fmt.Errorf("read live config for backup: %w", err)
	}

	dst := m.backupPath(n)
	if err := os.WriteFile(dst, data, 0640); err != nil {
		return fmt.Errorf("write backup .%d: %w", n, err)
	}
	return nil
}

// atomicWrite writes content to a temp file in the same directory and then
// renames it over the live path. This prevents partial writes from leaving
// the config file in a corrupt state.
func (m *ConfigFileManager) atomicWrite(content string) error {
	dir := dirOf(m.path)
	tmp, err := os.CreateTemp(dir, ".ocserv-conf-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Ensure temp file is removed on any failure path.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0640); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, m.path); err != nil {
		return fmt.Errorf("rename temp to config: %w", err)
	}

	committed = true
	return nil
}

// dirOf returns the directory containing path, using "." when path has no
// directory component so that os.CreateTemp receives a valid directory.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}
