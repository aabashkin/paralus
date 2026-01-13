# Relay Integration Guide

## Overview

Paralus includes a comprehensive relay integration system that enables secure, zero-trust access to Kubernetes clusters across different networks and cloud providers. This document describes the relay integration architecture, components, and how to use them.

## Architecture

The relay system follows a hub-and-spoke architecture:

- **Hub**: The Paralus core service acts as the central hub
- **Spokes**: Relay agents deployed in target Kubernetes clusters

### Components

#### 1. Relay Peer Service

Located in `server/relaypeerservice.go`, the relay peer service manages connections with relay agents.

**Key Features:**
- Maintains active connections with relay agents
- Handles peer discovery and health checks
- Manages relay-to-relay communication for multi-cluster scenarios
- Implements probe and survey mechanisms for cluster connectivity

**RPC Endpoints:**
- `RelayPeerHelloRPC`: Heartbeat mechanism to track active relays
- `RelayPeerProbeRPC`: Query for cluster connectivity information
- `RelayPeerSurveyRPC`: Broadcast queries to discover cluster connections

#### 2. Relay Audit Service

Located in `server/relayaudit.go`, this service handles audit logging for relay operations.

**Features:**
- Tracks kubectl command execution
- Logs API calls through the relay
- Supports both database and Elasticsearch storage backends
- Provides audit trail for compliance requirements

#### 3. Relay Peering Client

Located in `pkg/sentry/peering/peering.go`, provides client-side functionality for relay agents.

**Functions:**
- `ClientHelloRPC`: Sends periodic heartbeats to the core service
- `ClientProbeRPC`: Queries for peer relay information
- `ClientSurveyRPC`: Responds to survey requests about cluster connectivity

## Configuration

### Environment Variables

```bash
# Relay peering
SENTRY_PEERING_HOST="peering.sentry.paralus.local:10001"

# Relay connectors
CORE_RELAY_CONNECTOR_HOST="*.core-connector.relay.paralus.local:10002"
CORE_RELAY_USER_HOST="*.user.relay.paralus.local:10002"

# Bootstrap
SENTRY_BOOTSTRAP_ADDR="console.paralus.dev:443"
BOOTSTRAP_KEK="paralus"

# Relay image
RELAY_IMAGE="paralusio/relay:v0.1.0"

# Audit configuration
AUDIT_LOG_STORAGE="database"  # or "elasticsearch"
ES_END_POINT="http://127.0.0.1:9200"
RELAY_AUDITS_ES_INDEX_PREFIX="ralog-relay"
RELAY_COMMANDS_ES_INDEX_PREFIX="ralog-prompt"
```

## Communication Flow

### 1. Cluster Registration

```
User/Admin -> Paralus API -> Bootstrap Service
                              |
                              v
                        Generate Tokens
                              |
                              v
                        Relay Agent Template
```

### 2. Agent Connection

```
Relay Agent -> (Outbound TLS) -> Relay Peer Service
                                       |
                                       v
                                  Hello RPC (Heartbeat)
                                       |
                                       v
                                  Register in RelayMap
```

### 3. User Access

```
User kubectl -> Paralus API -> Relay Server -> Relay Agent -> K8s API
                    |
                    v
              Audit Logging
```

## Security

### TLS Certificates

The relay system uses mutual TLS for all communications:

- **Server Certificates**: Generated via `peering.GetPeeringServerCreds()`
- **Client Certificates**: Issued during bootstrap process
- **CA Certificate**: Paralus acts as the certificate authority

### Authentication

1. **Relay Authentication**: Each relay has a unique UUID and token
2. **User Authentication**: Handled by Paralus auth system before relay forwarding
3. **Organization Isolation**: Relays are isolated by organization (OU in certificate)

## API Reference

### Relay Peer Service (gRPC)

```protobuf
service RelayPeerService {
  rpc RelayPeerHelloRPC(stream PeerHelloRequest) returns (stream PeerHelloResponse);
  rpc RelayPeerProbeRPC(stream PeerProbeRequest) returns (stream PeerProbeResponse);
  rpc RelayPeerSurveyRPC(stream PeerSurveyResponse) returns (stream PeerSurveyRequest);
}
```

### Relay Audit Service (REST)

```
GET /event/v1/{project}/audit/relay
GET /event/v1/audit/relay
```

Query parameters:
- `type`: Filter by audit type (api/command)
- `user`: Filter by username
- `cluster`: Filter by cluster name
- `timefrom`: Start time for query
- `projects`: Array of project IDs

## References

- [Paralus Architecture Documentation](https://www.paralus.io/docs/architecture/)
- [Relay Repository](https://github.com/paralus/relay)
- [Bootstrap Service Documentation](https://www.paralus.io/docs/bootstrap/)
- [Audit Logging](https://www.paralus.io/docs/usage/audit-logs/)
