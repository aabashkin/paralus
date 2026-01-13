package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/paralus/paralus/internal/dao"
	"github.com/paralus/paralus/internal/models"
	"github.com/paralus/paralus/pkg/audit"
	"github.com/paralus/paralus/pkg/query"
	"github.com/uptrace/bun"
)

// SessionRecording represents a recorded terminal session
type SessionRecording struct {
	SessionID string    `json:"session_id"`
	Username  string    `json:"username"`
	Cluster   string    `json:"cluster"`
	Namespace string    `json:"namespace"`
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	Project   string    `json:"project"`
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	// Transcript stores the session output as array of chunks
	Transcript []SessionChunk `json:"transcript,omitempty"`
}

// SessionChunk represents a chunk of session data
type SessionChunk struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // stdout, stderr, stdin
	Data      string    `json:"data"`
}

// SessionRecordingService provides session recording functionality
type SessionRecordingService interface {
	// StartSession records the beginning of a session
	StartSession(ctx context.Context, session *SessionRecording) error
	// RecordChunk records a chunk of session data
	RecordChunk(ctx context.Context, sessionID string, chunk *SessionChunk) error
	// EndSession records the end of a session
	EndSession(ctx context.Context, sessionID string, exitCode int) error
	// GetSession retrieves a session recording
	GetSession(ctx context.Context, sessionID string) (*SessionRecording, error)
	// ListSessions lists session recordings with filters
	ListSessions(ctx context.Context, filters query.QueryFilters) ([]*SessionRecording, error)
}

type sessionRecordingDatabaseService struct {
	db *bun.DB
}

// NewSessionRecordingService creates a new session recording service
func NewSessionRecordingService(db *bun.DB) SessionRecordingService {
	return &sessionRecordingDatabaseService{
		db: db,
	}
}

// StartSession records the beginning of a session
func (s *sessionRecordingDatabaseService) StartSession(ctx context.Context, session *SessionRecording) error {
	// Set start time if not already set
	if session.StartTime.IsZero() {
		session.StartTime = time.Now()
	}

	// Create audit log entry
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	auditLog := &models.AuditLog{
		Tag:  audit.KUBECTL_SESSION,
		Time: session.StartTime,
		Data: data,
	}

	_, err = s.db.NewInsert().Model(auditLog).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert session start: %w", err)
	}

	return nil
}

// RecordChunk records a chunk of session data
func (s *sessionRecordingDatabaseService) RecordChunk(ctx context.Context, sessionID string, chunk *SessionChunk) error {
	// Get the existing session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Append the chunk to the transcript
	if chunk.Timestamp.IsZero() {
		chunk.Timestamp = time.Now()
	}
	session.Transcript = append(session.Transcript, *chunk)

	// Update the audit log
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	_, err = s.db.NewUpdate().
		Model((*models.AuditLog)(nil)).
		Set("data = ?", data).
		Where("tag = ?", audit.KUBECTL_SESSION).
		Where("data->>'session_id' = ?", sessionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// EndSession records the end of a session
func (s *sessionRecordingDatabaseService) EndSession(ctx context.Context, sessionID string, exitCode int) error {
	// Get the existing session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Set end time and exit code
	session.EndTime = time.Now()
	session.ExitCode = exitCode

	// Update the audit log
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	_, err = s.db.NewUpdate().
		Model((*models.AuditLog)(nil)).
		Set("data = ?", data).
		Where("tag = ?", audit.KUBECTL_SESSION).
		Where("data->>'session_id' = ?", sessionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// GetSession retrieves a session recording
func (s *sessionRecordingDatabaseService) GetSession(ctx context.Context, sessionID string) (*SessionRecording, error) {
	var log models.AuditLog
	err := s.db.NewSelect().
		Model(&log).
		Where("tag = ?", audit.KUBECTL_SESSION).
		Where("data->>'session_id' = ?", sessionID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session SessionRecording
	err = json.Unmarshal(log.Data, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &session, nil
}

// ListSessions lists session recordings with filters
func (s *sessionRecordingDatabaseService) ListSessions(ctx context.Context, filters query.QueryFilters) ([]*SessionRecording, error) {
	var logs []models.AuditLog
	sq := s.db.NewSelect().Model(&logs).
		Where("tag = ?", audit.KUBECTL_SESSION)

	// Apply filters
	if filters.GetUser() != "" {
		sq.Where("data->>'username' = ?", filters.GetUser())
	}
	if filters.GetCluster() != "" {
		sq.Where("data->>'cluster' = ?", filters.GetCluster())
	}
	if filters.GetNamespace() != "" {
		sq.Where("data->>'namespace' = ?", filters.GetNamespace())
	}
	if len(filters.GetProjects()) > 0 {
		sq.Where("data->>'project' IN (?)", bun.In(filters.GetProjects()))
	}
	if filters.GetTimefrom() != "" {
		sq.Where("time >= ?", filters.GetTimefrom())
	}

	err := sq.Order("time DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*SessionRecording, 0, len(logs))
	for _, log := range logs {
		var session SessionRecording
		err = json.Unmarshal(log.Data, &session)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}
