package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paralus/paralus/pkg/query"
)

func TestSessionRecording(t *testing.T) {
	// This test requires a database connection
	// Skip if database is not available
	db, err := getDB()
	if err != nil {
		t.Skip("database not available:", err)
	}

	svc := NewSessionRecordingService(db)
	ctx := context.Background()

	// Create a test session
	sessionID := uuid.New().String()
	session := &SessionRecording{
		SessionID: sessionID,
		Username:  "test-user",
		Cluster:   "test-cluster",
		Namespace: "default",
		Pod:       "test-pod",
		Container: "test-container",
		Project:   "test-project",
		Command:   "/bin/sh",
		StartTime: time.Now(),
	}

	// Test StartSession
	err = svc.StartSession(ctx, session)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	// Test RecordChunk
	chunk := &SessionChunk{
		Timestamp: time.Now(),
		Stream:    "stdout",
		Data:      "test output",
	}
	err = svc.RecordChunk(ctx, sessionID, chunk)
	if err != nil {
		t.Fatalf("RecordChunk failed: %v", err)
	}

	// Test GetSession
	retrieved, err := svc.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if retrieved.SessionID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, retrieved.SessionID)
	}
	if len(retrieved.Transcript) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(retrieved.Transcript))
	}

	// Test EndSession
	err = svc.EndSession(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}

	// Test ListSessions
	filters := query.NewQueryFilter()
	filters.SetUser("test-user")
	sessions, err := svc.ListSessions(ctx, filters)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session")
	}
}
