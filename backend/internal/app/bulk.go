package app

// BulkResult reports partial-success results for batch operations.
type BulkResult struct {
	Total     int         `json:"total"`
	Succeeded int         `json:"succeeded"`
	Failed    int         `json:"failed"`
	Errors    []BulkError `json:"errors"`
}

// BulkError describes a single item failure within a bulk operation.
type BulkError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// MaxBulkBatchSize is the maximum number of items allowed in a single bulk request.
const MaxBulkBatchSize = 100
