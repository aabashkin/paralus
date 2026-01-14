# Session Recording Feature

## Overview

The session recording feature captures interactive terminal sessions when DevOps engineers access pods using `kubectl exec`. This provides audit trails for debugging sessions and compliance purposes.

## Architecture

The session recording mechanism consists of four main components:

1. **Session Recording Service** (`pkg/service/session_recording.go`) - Core service for managing sessions
2. **Session Recording Interceptor** (`pkg/service/session_recording_interceptor.go`) - I/O stream wrapper for easy integration
3. **Database Storage** (uses existing `audit_logs` table)
4. **Integration with Relay Audit API**

## Quick Start for Relay Components

### Option 1: Using the Interceptor (Recommended)

The `SessionRecordingInterceptor` provides the easiest way to integrate session recording:

```go
import "github.com/paralus/paralus/pkg/service"

// 1. Create session metadata
session := &service.SessionRecording{
    Username:  "user@example.com",
    Cluster:   "prod-cluster",
    Namespace: "default",
    Pod:       "app-pod-123",
    Container: "main",
    Project:   "my-project",
    Command:   "/bin/bash",
}

// 2. Create interceptor (automatically starts session)
interceptor, err := service.NewSessionRecordingInterceptor(svc, session)
if err != nil {
    return err
}

// 3. Wrap your I/O streams
recordedStdin := interceptor.WrapStdin(stdin)
recordedStdout := interceptor.WrapStdout(stdout)
recordedStderr := interceptor.WrapStderr(stderr)

// 4. Use wrapped streams in your exec handler
exitCode := executeKubectlExec(recordedStdin, recordedStdout, recordedStderr)

// 5. End the session
interceptor.End(exitCode)
```

### Option 2: Direct Service Usage

For more control, use the service directly:

```go
// 1. Start session
session := &service.SessionRecording{
    SessionID: uuid.New().String(),
    Username:  "user@example.com",
    Cluster:   "prod-cluster",
    Pod:       "app-pod-123",
    Command:   "/bin/sh",
}
err := svc.StartSession(ctx, session)

// 2. Record I/O chunks
chunk := &service.SessionChunk{
    Timestamp: time.Now(),
    Stream:    "stdout",
    Data:      output,
}
err = svc.RecordChunk(ctx, sessionID, chunk)

// 3. End session
err = svc.EndSession(ctx, sessionID, exitCode)
```

## Integration Examples

### WebSocket Handler

```go
func handleWebSocketExec(ctx context.Context, wsConn *websocket.Conn) {
    metadata := extractSessionMetadata(ctx)
    interceptor, _ := service.NewSessionRecordingInterceptor(svc, metadata)
    defer interceptor.End(0)
    
    // Create pipes
    stdinR, stdinW := io.Pipe()
    stdoutR, stdoutW := io.Pipe()
    
    // Wrap with recording
    recordedStdin := interceptor.WrapStdin(stdinR)
    recordedStdout := interceptor.WrapStdout(stdoutW)
    
    // Connect WebSocket to pipes (goroutines)
    go streamWSToStdin(wsConn, stdinW)
    go streamStdoutToWS(stdoutR, wsConn)
    
    // Execute with recorded streams
    executeRemoteCommand(recordedStdin, recordedStdout, nil)
}
```

### SPDY/HTTP2 Handler (Kubernetes Client-Go)

```go
import (
    "k8s.io/client-go/tools/remotecommand"
)

func handleSPDYExec(ctx context.Context, config *rest.Config) {
    metadata := extractSessionMetadata(ctx)
    interceptor, _ := service.NewSessionRecordingInterceptor(svc, metadata)
    defer interceptor.End(0)
    
    // Get SPDY executor
    exec, _ := remotecommand.NewSPDYExecutor(config, "POST", url)
    
    // Wrap streams
    recordedStdin := interceptor.WrapStdin(clientStdin)
    recordedStdout := interceptor.WrapStdout(clientStdout)
    recordedStderr := interceptor.WrapStderr(clientStderr)
    
    // Execute with wrapped streams
    exec.Stream(remotecommand.StreamOptions{
        Stdin:  recordedStdin,
        Stdout: recordedStdout,
        Stderr: recordedStderr,
        Tty:    true,
    })
}
```

## How It Works

### The Interceptor Pattern

The `SessionRecordingInterceptor` wraps `io.Reader` and `io.Writer` interfaces:

1. **RecordingWriter**: Captures data written to stdout/stderr
   - Writes data to underlying stream (normal operation)
   - Asynchronously records data to session transcript
   - Non-blocking - recording failures don't affect exec session

2. **RecordingReader**: Captures data read from stdin
   - Reads data from underlying stream (normal operation)
   - Asynchronously records data to session transcript
   - Non-blocking - recording failures don't affect exec session

3. **Thread-safe**: Uses mutex for concurrent chunk recording

### Data Flow

```
Client → Stdin Reader → RecordingReader → Pod
                            ↓
                        Database

Pod → Stdout Writer → RecordingWriter → Client
                          ↓
                      Database

Pod → Stderr Writer → RecordingWriter → Client
                          ↓
                      Database
```

## API Access

Sessions are accessible via relay audit API with `auditType: "RelaySession"`

### Query Sessions

```bash
GET /event/v1/{project}/audit/relay?auditType=RelaySession
```

Filters:
- `user` - Filter by username
- `cluster` - Filter by cluster name
- `namespace` - Filter by namespace
- `timefrom` - Filter by time range
- `projects` - Filter by projects

### Get Specific Session

```bash
GET /event/v1/{project}/audit/session/{sessionID}
```

## Database Schema

Sessions stored in `audit_logs` table:
- `tag`: `"kubectl_session"`
- `time`: Session start timestamp
- `data`: JSON with SessionRecording structure

```json
{
  "session_id": "uuid",
  "username": "user@example.com",
  "cluster": "prod",
  "namespace": "default",
  "pod": "app-123",
  "container": "main",
  "project": "my-project",
  "command": "/bin/bash",
  "start_time": "2024-01-13T10:00:00Z",
  "end_time": "2024-01-13T10:05:30Z",
  "exit_code": 0,
  "transcript": [
    {
      "timestamp": "2024-01-13T10:00:01Z",
      "stream": "stdin",
      "data": "ls -la\n"
    },
    {
      "timestamp": "2024-01-13T10:00:01Z",
      "stream": "stdout",
      "data": "total 16\ndrwxr-xr-x..."
    }
  ]
}
```

## Performance

- **Overhead**: <100ms per operation, non-blocking
- **Storage**: ~1KB per command, ~100KB per 15-min session
- **Scalability**: Horizontally scalable with database
- **Reliability**: Recording failures don't affect exec sessions

## Security Considerations

1. **Access Control**: Sessions inherit audit log permissions
2. **Data Sensitivity**: Transcripts may contain sensitive data
3. **Retention**: Configure policies per organization requirements
4. **Encryption**: Uses existing database encryption

## Testing

See examples in:
- `pkg/service/session_recording_interceptor_test.go` - Unit tests
- `pkg/service/examples_session_recording_test.go` - Integration examples

## Troubleshooting

### Session Not Recorded

Check:
1. Service properly initialized in `main.go`
2. Database connection working
3. Interceptor created successfully
4. `End()` called to finalize session

### Missing Transcript Data

Check:
1. Streams properly wrapped with interceptor
2. Data flowing through wrapped streams
3. No errors in recording service logs

### Performance Issues

If recording impacts performance:
1. Check database performance
2. Consider batching chunk writes
3. Verify non-blocking behavior

## Future Enhancements

- Compression for large transcripts
- Real-time streaming to SIEM
- Session playback UI
- Export to asciinema format
- Pattern-based alerting
