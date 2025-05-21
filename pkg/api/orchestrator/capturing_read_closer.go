package orchestrator

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// CapturingReadCloser wraps an io.ReadCloser, capturing all data read from it
// into an internal buffer, while still passing the data through.
// It also handles calling finalizeTransaction upon Close for streaming requests.
type CapturingReadCloser struct {
	originalReader io.ReadCloser
	buffer         *bytes.Buffer
	mu             sync.Mutex
	finalized      bool

	// Fields required for finalizeTransaction
	service  *Service
	ctx      context.Context
	tx       *TransactionRecord
	latency  int64
	provider *ProviderInfo
}

// NewCapturingReadCloser creates a new CapturingReadCloser.
func NewCapturingReadCloser(r io.ReadCloser, service *Service, ctx context.Context, tx *TransactionRecord, latency int64, provider *ProviderInfo) *CapturingReadCloser {
	return &CapturingReadCloser{
		originalReader: r,
		buffer:         new(bytes.Buffer),
		service:        service,
		ctx:            ctx,
		tx:             tx,
		latency:        latency,
		provider:       provider,
	}
}

// Read reads from the original reader, passes the data to p,
// and also writes a copy to the internal buffer.
func (crc *CapturingReadCloser) Read(p []byte) (n int, err error) {
	n, err = crc.originalReader.Read(p)
	if n > 0 {
		crc.buffer.Write(p[:n])
	}
	return n, err
}

// Close closes the original reader and then triggers finalizeTransaction.
func (crc *CapturingReadCloser) Close() error {
	// Close the underlying stream first.
	// The error from closing the original reader should be returned to the caller (Echo).
	closeErr := crc.originalReader.Close()

	crc.mu.Lock()
	if !crc.finalized {
		crc.finalized = true // Mark as finalized early to prevent re-entry if finalizeTransaction panics or is slow
		crc.mu.Unlock()      // Unlock before calling finalizeTransaction to avoid holding lock during potentially long operation

		// The original request (OpenAIRequest) is expected to be in crc.ctx
		// finalizeTransaction will use crc (this instance) as the 'responseForTx' argument.
		if err := crc.service.finalizeTransaction(crc.ctx, crc.tx, crc, crc.latency, crc.provider);
		// Log errors from finalizeTransaction as it's a background process post-client-response.
		err != nil {
			crc.service.logger.Error("Error during finalizeTransaction in CapturingReadCloser.Close: %v", err)
		}
	} else {
		crc.mu.Unlock() // Unlock if already finalized
	}

	return closeErr
}

// GetCapturedData returns the data captured in the internal buffer as a string.
func (crc *CapturingReadCloser) GetCapturedData() string {
	// It's possible GetCapturedData could be called by finalizeTransaction while Read is still writing.
	// However, by the time Close() is called (which triggers finalizeTransaction), Read() should have completed.
	// For safety, if direct access to buffer from another goroutine (finalizeTransaction via Close) is a concern,
	// this could also be mutex-protected, but typically buffer reads are safe if writes have ceased.
	return crc.buffer.String()
}
