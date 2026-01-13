# Server Components

This directory contains the gRPC and HTTP server implementations for Paralus.

## Relay Components

### Relay Peer Service (`relaypeerservice.go`)

The relay peer service manages connections with relay agents deployed in target Kubernetes clusters.

**Key Functions:**
- `NewRelayPeerService()`: Creates a new relay peer service instance
- `RelayPeerHelloRPC()`: Handles heartbeat messages from relay agents
- `RelayPeerProbeRPC()`: Handles probe requests for cluster connectivity
- `RelayPeerSurveyRPC()`: Handles survey requests to discover cluster connections
- `RunRelaySurveyHandler()`: Background handler for survey broadcast processing

**Data Structures:**
```go
type relayPeerService struct {
    ServiceUUID       string                              // Unique service ID
    RelayMap          map[string]map[string]*relayObject  // OU -> RelayUUID -> RelayObject
    surveyBroadCast   chan surveyBroadCastRequest        // Survey broadcast channel
    surveyCacheExpiry time.Duration                       // Cache TTL
    peerServiceCache  *ristretto.Cache                    // Peer connection cache
}

type relayObject struct {
    timeStamp         int64                                   // Last heartbeat timestamp
    refCnt            uint8                                   // Reference count
    relayip           string                                  // IP address
    ou                string                                  // Organization unit
    probeReplyChnl    chan sentryrpc.PeerProbeResponse       // Probe reply channel
    surveyRequestChnl chan sentryrpc.PeerSurveyRequest      // Survey request channel
}
```

### Relay Audit Service (`relayaudit.go`)

Handles audit logging for relay operations including API calls and kubectl commands.

**Key Functions:**
- `NewRelayAuditServer()`: Creates a new relay audit server
- `GetRelayAudit()`: Retrieves relay audit logs for a project
- `GetRelayAuditByProjects()`: Retrieves relay audit logs across multiple projects

**Supported Audit Types:**
- `RelayAPIAuditType`: API calls through the relay
- `RelayCommandsAuditType`: kubectl commands executed

### Relay Peer Client (`relaypeerclient.go`)

Client-side implementation for relay agents to communicate with the relay peer service.

**Key Functions:**
- Connection management
- Heartbeat handling
- Probe and survey client implementations

## Integration

The relay services are integrated into the main server in `main.go`:

```go
func runRelayPeerRPC(wg *sync.WaitGroup, ctx context.Context) {
    // 1. Fetch peering server credentials
    cert, key, ca, err := peering.GetPeeringServerCreds(...)
    
    // 2. Create relay peer service
    relayPeerService, err := server.NewRelayPeerService()
    
    // 3. Create secure gRPC server
    s, err := grpc.NewSecureServerWithPEM(cert, key, ca)
    
    // 4. Register services
    sentryrpc.RegisterRelayPeerServiceServer(s, relayPeerService)
    sentryrpc.RegisterClusterAuthorizationServiceServer(s, clusterAuthzServer)
    
    // 5. Start survey handler
    go server.RunRelaySurveyHandler(ctx.Done(), relayPeerService)
    
    // 6. Serve on rpcRelayPeeringPort (default: 10001)
    s.Serve(l)
}
```

## Testing

### Unit Tests

Run unit tests for server components:

```bash
go test ./server -v
```

### Integration Tests

The `relay_integration_test.go` file provides examples and templates for integration testing:

```bash
# Run with integration test setup
go test ./server -run Integration -v
```

Note: Integration tests are skipped by default and require a full Paralus setup including:
- PostgreSQL database
- Kratos identity service
- Relay agent(s)

### Manual Testing

To manually test the relay services:

1. Start Paralus with development mode enabled:
   ```bash
   DEV=true make run
   ```

2. Register a test cluster:
   ```bash
   # Use Paralus API or CLI to register a cluster
   pctl cluster register --name test-cluster
   ```

3. Deploy the relay agent to the target cluster:
   ```bash
   kubectl apply -f <agent-template.yaml>
   ```

4. Monitor relay connections:
   ```bash
   # Check relay peer service logs
   kubectl logs -f deployment/paralus -c paralus | grep relay
   ```

## Configuration

Relay services are configured via environment variables. See `main.go` for the complete list:

```bash
SENTRY_PEERING_HOST        # Relay peering endpoint
CORE_RELAY_CONNECTOR_HOST  # Relay connector endpoint
CORE_RELAY_USER_HOST       # Relay user endpoint
RELAY_IMAGE                # Docker image for relay agents
```

## Security

### TLS Configuration

Relay services use mutual TLS authentication:
- Server certificates are generated via `peering.GetPeeringServerCreds()`
- Client certificates are issued during cluster bootstrap
- Organization isolation via certificate OU field

### Authorization

- Relay connections are authenticated via client certificates
- User requests through relay are authorized by Paralus RBAC
- Audit logs capture all relay operations

## Monitoring

### Metrics

Key metrics to monitor:
- Number of active relays per organization
- Heartbeat frequency and latency
- Probe/survey response times
- Audit log volume

### Health Checks

The relay peer service performs automatic health checks:
- Relays not sending heartbeats for >5 minutes are marked stale
- Stale relays are periodically cleaned up
- Survey mechanism verifies cluster connectivity

## Troubleshooting

See `docs/RELAY_INTEGRATION.md` for comprehensive troubleshooting guide.

Common issues:
- Relay agent not connecting: Check network connectivity and certificates
- Probe failures: Verify cache expiry settings and relay health
- Audit logs missing: Check storage backend configuration

## Development

### Adding New Relay Features

1. Update proto definitions in `proto/rpc/sentry/relaypeer.proto`
2. Regenerate proto code: `make build-proto`
3. Implement server-side logic in `relaypeerservice.go`
4. Implement client-side logic in `pkg/sentry/peering/peering.go`
5. Add tests in `relay_integration_test.go`
6. Update documentation in `docs/RELAY_INTEGRATION.md`

### Code Style

Follow existing patterns:
- Use structured logging with `_log.Infow/Errorw`
- Handle context cancellation properly
- Use channels for async communication
- Implement proper mutex locking for shared state

## References

- [Relay Integration Guide](../docs/RELAY_INTEGRATION.md)
- [Proto Definitions](../proto/rpc/sentry/relaypeer.proto)
- [Peering Client](../pkg/sentry/peering/peering.go)
