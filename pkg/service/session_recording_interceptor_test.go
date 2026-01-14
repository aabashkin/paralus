package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestRecordingWriter(t *testing.T) {
	// Skip if database is not available
	db, err := getDB()
	if err != nil {
		t.Skip("database not available:", err)
	}

	svc := NewSessionRecordingService(db)
	
	session := &SessionRecording{
		Username:  "test-user",
		Cluster:   "test-cluster",
		Namespace: "default",
		Pod:       "test-pod",
		Container: "main",
		Project:   "test-project",
		Command:   "/bin/sh",
	}

	interceptor, err := NewSessionRecordingInterceptor(svc, session)
	if err != nil {
		t.Fatalf("NewSessionRecordingInterceptor failed: %v", err)
	}

	// Test stdout recording
	var buf bytes.Buffer
	stdout := interceptor.WrapStdout(&buf)
	
	testData := "Hello from stdout\n"
	n, err := stdout.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}
	
	// Verify data was written to underlying buffer
	if buf.String() != testData {
		t.Errorf("expected %q in buffer, got %q", testData, buf.String())
	}

	// End the session
	err = interceptor.End(0)
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Retrieve and verify the session
	retrieved, err := svc.GetSession(nil, interceptor.SessionID())
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(retrieved.Transcript) == 0 {
		t.Error("expected transcript to have entries")
	}

	// Check if our stdout data was recorded
	foundStdout := false
	for _, chunk := range retrieved.Transcript {
		if chunk.Stream == "stdout" && strings.Contains(chunk.Data, testData) {
			foundStdout = true
			break
		}
	}
	if !foundStdout {
		t.Error("stdout data was not recorded in transcript")
	}
}

func TestRecordingReader(t *testing.T) {
	// Skip if database is not available
	db, err := getDB()
	if err != nil {
		t.Skip("database not available:", err)
	}

	svc := NewSessionRecordingService(db)
	
	session := &SessionRecording{
		Username:  "test-user",
		Cluster:   "test-cluster",
		Namespace: "default",
		Pod:       "test-pod",
		Container: "main",
		Project:   "test-project",
		Command:   "/bin/sh",
	}

	interceptor, err := NewSessionRecordingInterceptor(svc, session)
	if err != nil {
		t.Fatalf("NewSessionRecordingInterceptor failed: %v", err)
	}

	// Test stdin recording
	testData := "echo hello\n"
	stdin := interceptor.WrapStdin(strings.NewReader(testData))
	
	buf := make([]byte, len(testData))
	n, err := stdin.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected to read %d bytes, read %d", len(testData), n)
	}

	// End the session
	err = interceptor.End(0)
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Retrieve and verify the session
	retrieved, err := svc.GetSession(nil, interceptor.SessionID())
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	// Check if our stdin data was recorded
	foundStdin := false
	for _, chunk := range retrieved.Transcript {
		if chunk.Stream == "stdin" && strings.Contains(chunk.Data, testData) {
			foundStdin = true
			break
		}
	}
	if !foundStdin {
		t.Error("stdin data was not recorded in transcript")
	}
}

func TestInterceptorWithMultipleStreams(t *testing.T) {
	// Skip if database is not available
	db, err := getDB()
	if err != nil {
		t.Skip("database not available:", err)
	}

	svc := NewSessionRecordingService(db)
	
	session := &SessionRecording{
		Username:  "test-user",
		Cluster:   "test-cluster",
		Namespace: "default",
		Pod:       "test-pod",
		Container: "main",
		Project:   "test-project",
		Command:   "/bin/sh",
	}

	interceptor, err := NewSessionRecordingInterceptor(svc, session)
	if err != nil {
		t.Fatalf("NewSessionRecordingInterceptor failed: %v", err)
	}

	// Write to stdout
	var stdoutBuf bytes.Buffer
	stdout := interceptor.WrapStdout(&stdoutBuf)
	stdout.Write([]byte("stdout message\n"))

	// Write to stderr
	var stderrBuf bytes.Buffer
	stderr := interceptor.WrapStderr(&stderrBuf)
	stderr.Write([]byte("stderr message\n"))

	// Read from stdin
	stdin := interceptor.WrapStdin(strings.NewReader("stdin message\n"))
	buf := make([]byte, 100)
	stdin.Read(buf)

	// End the session
	err = interceptor.End(0)
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Retrieve and verify all streams were recorded
	retrieved, err := svc.GetSession(nil, interceptor.SessionID())
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	streamCounts := make(map[string]int)
	for _, chunk := range retrieved.Transcript {
		streamCounts[chunk.Stream]++
	}

	if streamCounts["stdout"] == 0 {
		t.Error("stdout was not recorded")
	}
	if streamCounts["stderr"] == 0 {
		t.Error("stderr was not recorded")
	}
	if streamCounts["stdin"] == 0 {
		t.Error("stdin was not recorded")
	}
}
