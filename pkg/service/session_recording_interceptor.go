package service

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionRecordingInterceptor provides a wrapper for recording exec sessions
type SessionRecordingInterceptor struct {
	svc       SessionRecordingService
	sessionID string
	mu        sync.Mutex
}

// NewSessionRecordingInterceptor creates a new interceptor for a session
func NewSessionRecordingInterceptor(svc SessionRecordingService, session *SessionRecording) (*SessionRecordingInterceptor, error) {
	// Generate session ID if not provided
	if session.SessionID == "" {
		session.SessionID = uuid.New().String()
	}

	// Start the session
	ctx := context.Background()
	if err := svc.StartSession(ctx, session); err != nil {
		return nil, err
	}

	return &SessionRecordingInterceptor{
		svc:       svc,
		sessionID: session.SessionID,
	}, nil
}

// RecordingWriter wraps an io.Writer to capture data for session recording
type RecordingWriter struct {
	underlying io.Writer
	interceptor *SessionRecordingInterceptor
	stream      string
}

// Write implements io.Writer and records the data
func (w *RecordingWriter) Write(p []byte) (n int, err error) {
	// Write to underlying writer first
	n, err = w.underlying.Write(p)
	
	// Record the chunk (best effort, don't fail the write if recording fails)
	if n > 0 {
		chunk := &SessionChunk{
			Timestamp: time.Now(),
			Stream:    w.stream,
			Data:      string(p[:n]),
		}
		_ = w.interceptor.recordChunk(chunk)
	}
	
	return n, err
}

// RecordingReader wraps an io.Reader to capture stdin data
type RecordingReader struct {
	underlying io.Reader
	interceptor *SessionRecordingInterceptor
}

// Read implements io.Reader and records stdin data
func (r *RecordingReader) Read(p []byte) (n int, err error) {
	n, err = r.underlying.Read(p)
	
	// Record stdin (best effort)
	if n > 0 {
		chunk := &SessionChunk{
			Timestamp: time.Now(),
			Stream:    "stdin",
			Data:      string(p[:n]),
		}
		_ = r.interceptor.recordChunk(chunk)
	}
	
	return n, err
}

// WrapStdout wraps stdout writer for recording
func (i *SessionRecordingInterceptor) WrapStdout(w io.Writer) io.Writer {
	return &RecordingWriter{
		underlying:  w,
		interceptor: i,
		stream:      "stdout",
	}
}

// WrapStderr wraps stderr writer for recording
func (i *SessionRecordingInterceptor) WrapStderr(w io.Writer) io.Writer {
	return &RecordingWriter{
		underlying:  w,
		interceptor: i,
		stream:      "stderr",
	}
}

// WrapStdin wraps stdin reader for recording
func (i *SessionRecordingInterceptor) WrapStdin(r io.Reader) io.Reader {
	return &RecordingReader{
		underlying:  r,
		interceptor: i,
	}
}

// recordChunk is a helper to record chunks with proper locking
func (i *SessionRecordingInterceptor) recordChunk(chunk *SessionChunk) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	ctx := context.Background()
	return i.svc.RecordChunk(ctx, i.sessionID, chunk)
}

// End finalizes the session recording
func (i *SessionRecordingInterceptor) End(exitCode int) error {
	ctx := context.Background()
	return i.svc.EndSession(ctx, i.sessionID, exitCode)
}

// SessionID returns the session ID
func (i *SessionRecordingInterceptor) SessionID() string {
	return i.sessionID
}
