package app

// Test-only exports for helper functions used in health_service_test.go.

// StorageStatusForTest exposes storageStatus for external test packages.
func StorageStatusForTest(size, warnThreshold, critThreshold int64) string {
	return storageStatus(size, warnThreshold, critThreshold)
}

// MaskPathForTest exposes maskPath for external test packages.
func MaskPathForTest(path string, segments int) string {
	return maskPath(path, segments)
}

// FileSizeForTest exposes fileSize for external test packages.
func FileSizeForTest(path string) int64 {
	return fileSize(path)
}

// DirSizeForTest exposes dirSize for external test packages.
func DirSizeForTest(path string) int64 {
	return dirSize(path)
}
