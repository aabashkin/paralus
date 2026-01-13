# Session Recording Feature

## Overview

The session recording feature captures interactive terminal sessions when DevOps engineers access pods using `kubectl exec`. This provides audit trails for debugging sessions and compliance purposes.

## Architecture

The session recording mechanism consists of three main components:

1. **Session Recording Service** (`pkg/service/session_recording.go`)
2. **Database Storage** (uses existing `audit_logs` table)
3. **Integration with Relay Audit API**

## Usage

### For Relay Components

When handling kubectl exec requests, relay components should:

1. Generate a unique session ID
2. Call `StartSession()` when exec session begins
3. Call `RecordChunk()` for each I/O event (stdin/stdout/stderr)
4. Call `EndSession()` when session terminates

### API Access

Sessions are accessible via relay audit API with `auditType: "RelaySession"`

See full documentation in the file.
