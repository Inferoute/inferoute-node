package orchestrator

import (
	"bytes"
	"io"
)

// CapturingReadCloser wraps an io.ReadCloser, capturing all data read from it
// into an internal buffer, while still passing the data through.
type CapturingReadCloser struct {
	originalReader io.ReadCloser
	buffer         *bytes.Buffer
}

// NewCapturingReadCloser creates a new CapturingReadCloser.
func NewCapturingReadCloser(r io.ReadCloser) *CapturingReadCloser {
	return &CapturingReadCloser{
		originalReader: r,
		buffer:         new(bytes.Buffer),
	}
}

// Read reads from the original reader, passes the data to p,
// and also writes a copy to the internal buffer.
func (crc *CapturingReadCloser) Read(p []byte) (n int, err error) {
	n, err = crc.originalReader.Read(p)
	if n > 0 {
		// Write to internal buffer, ignore error as buffer.Write should not fail with non-nil p
		crc.buffer.Write(p[:n])
	}
	return n, err
}

// Close closes the original reader.
func (crc *CapturingReadCloser) Close() error {
	return crc.originalReader.Close()
}

// GetCapturedData returns the data captured in the internal buffer as a string.
func (crc *CapturingReadCloser) GetCapturedData() string {
	return crc.buffer.String()
}
