package logs

import "context"

// FakeLogReader is a test double that returns pre-configured results.
type FakeLogReader struct {
	available bool
	result    *ReadResult
	err       error
}

// NewFakeLogReader returns a FakeLogReader.
func NewFakeLogReader(available bool, result *ReadResult, err error) *FakeLogReader {
	return &FakeLogReader{available: available, result: result, err: err}
}

// Available returns the pre-configured availability.
func (f *FakeLogReader) Available(_ context.Context) bool {
	return f.available
}

// ReadLines returns the pre-configured result.
func (f *FakeLogReader) ReadLines(_ context.Context, _ ReadOpts) (*ReadResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}
