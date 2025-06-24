# ProxWarden

ProxWarden is a Golang CLI tool and systemd service for automating failover of critical Proxmox containers based on health checks and manual triggers.

## Features

- **Automated Health Monitoring**: Continuous monitoring of container health using TCP, HTTP, and ICMP checks
- **Backup-Based Failover**: Automatic restoration of containers from backups on healthy nodes when failures are detected
- **Manual Failover Control**: CLI commands for manual failover operations
- **Flexible Configuration**: YAML-based configuration with support for multiple containers and health check types
- **Systemd Integration**: Runs as a systemd service with proper lifecycle management
- **Extensible Architecture**: Designed for easy extension with additional automations and interfaces
- **Comprehensive Logging**: Structured logging with configurable levels and formats
- **Security Hardened**: Systemd service with security restrictions and proper user isolation

## Quick Start

### Prerequisites

- Go 1.21 or later
- Proxmox VE cluster with shared backup storage
- Linux system with systemd (for daemon mode)
- Network connectivity between all Proxmox nodes

### Installation

1. **Clone and build:**
   ```bash
   git clone https://github.com/jbutlerdev/proxwarden.git
   cd proxwarden
   make build
   ```

2. **Install system-wide:**
   ```bash
   make install
   ```

3. **Configure:**
   ```bash
   sudo cp configs/proxwarden.example.yaml /etc/proxwarden/proxwarden.yaml
   sudo nano /etc/proxwarden/proxwarden.yaml
   ```

4. **Start the service:**
   ```bash
   sudo systemctl enable --now proxwarden
   ```

## Configuration

ProxWarden uses YAML configuration files. The default location is `/etc/proxwarden/proxwarden.yaml`.

### Example Configuration

```yaml
# Proxmox VE connection settings
proxmox:
  endpoint: "https://your-proxmox-server:8006"
  username: "root@pam"
  password: "your-password"
  # Or use API tokens (recommended):
  # token_id: "your-token-id"
  # secret: "your-secret"
  insecure: false

# Container monitoring
monitoring:
  interval: 30s
  timeout: 10s
  failure_threshold: 3
  containers:
    - id: 100
      name: "web-server"
      priority: 1
      failover_nodes: ["node2", "node3"]
      health_checks:
        - type: "tcp"
          target: "192.168.1.100"
          port: 80
          timeout: 5s
        - type: "http"
          target: "192.168.1.100"
          port: 80
          path: "/health"
          timeout: 10s

# Failover behavior
failover:
  auto_failover: true
  max_retries: 3
  retry_delay: 5s
  pre_failover_hooks:
    - "/usr/local/bin/notify-failover.sh"
  post_failover_hooks:
    - "/usr/local/bin/update-dns.sh"

# Logging
logging:
  level: "info"
  format: "json"
```

## CLI Usage

### Daemon Mode
```bash
# Run as daemon
proxwarden daemon

# Run with debug logging
proxwarden daemon --debug --log-level debug
```

### Manual Failover
```bash
# Trigger failover for container 100
proxwarden failover trigger 100

# Failover to specific node
proxwarden failover trigger 100 --target-node node2

# Force failover even if container is healthy
proxwarden failover trigger 100 --force
```

### Status Checking
```bash
# Show container status
proxwarden status

# JSON output
proxwarden status --json
```

## Backup-Based Failover Process

ProxWarden uses a backup/restore approach for failover instead of live migration, ensuring data consistency and compatibility across different storage types:

### Failover Workflow

1. **Health Monitoring**: Continuous monitoring detects container failure
2. **Backup Creation**: Creates fresh backup or uses latest existing backup  
3. **Original Container Shutdown**: Attempts to gracefully stop the failed container
4. **Backup Restoration**: Restores container from backup on target healthy node
5. **Container Startup**: Starts the restored container on the new node
6. **Hook Execution**: Runs post-failover hooks (DNS updates, notifications)

### Benefits of Backup-Based Failover

- **Data Consistency**: Ensures clean state restoration from known-good backups
- **Storage Independence**: Works across different storage types and configurations
- **Reduced Complexity**: Avoids live migration complexities and potential data corruption
- **Audit Trail**: Maintains backup history for troubleshooting and compliance

### Requirements

- **Shared Backup Storage**: All nodes must access the same backup storage
- **Sufficient Space**: Backup storage must accommodate container backups
- **Network Connectivity**: Nodes must communicate for backup/restore operations

## Health Check Types

ProxWarden supports multiple health check types:

- **TCP**: Tests TCP connectivity to a port
- **HTTP/HTTPS**: Makes HTTP requests and checks response codes
- **ICMP/Ping**: Tests network reachability (requires elevated permissions)

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Interface │    │  Systemd Service │    │   Config Mgmt   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Daemon Core    │
                    └─────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ Health Monitor  │ │ Failover Engine │ │ Proxmox API     │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

### Core Components

- **Health Monitor**: Continuously monitors container health using configurable checks
- **Failover Engine**: Orchestrates container migrations and manages failover logic
- **Proxmox API Client**: Handles all Proxmox VE API interactions
- **Configuration Manager**: Manages YAML-based configuration with validation
- **CLI Interface**: Provides command-line interface for operations and management

## Development

### Building

```bash
# Build for current platform
make build

# Build for Linux
make build-linux

# Build for all platforms
make build-all

# Development mode with auto-reload
make dev
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection
make test-race

# Run all checks (format, vet, lint, test)
make check
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet
```

## Systemd Service

The systemd service is configured with security hardening:

- Runs as dedicated `proxwarden` user
- Restricted filesystem access
- No new privileges
- Memory protection
- System call filtering

Service management:
```bash
# Start/stop
sudo systemctl start proxwarden
sudo systemctl stop proxwarden

# Enable/disable auto-start
sudo systemctl enable proxwarden
sudo systemctl disable proxwarden

# View logs
sudo journalctl -u proxwarden -f

# View status
sudo systemctl status proxwarden
```

## Security Considerations

- Use API tokens instead of passwords when possible
- Restrict network access to Proxmox API endpoints
- Regular rotation of API credentials
- Monitor service logs for suspicious activities
- Keep ProxWarden updated

## Troubleshooting

### Common Issues

1. **Connection refused to Proxmox API**
   - Check endpoint URL and port
   - Verify SSL/TLS configuration
   - Check firewall rules

2. **Authentication failures**
   - Verify credentials
   - Check user permissions in Proxmox
   - Ensure API tokens are valid

3. **Health checks failing**
   - Verify network connectivity
   - Check container IP addresses
   - Validate health check configuration

4. **Failover not working**
   - Check target node availability
   - Verify container storage accessibility
   - Review Proxmox cluster status

### Debugging

Enable debug logging:
```bash
proxwarden daemon --debug --log-level debug
```

Check service logs:
```bash
sudo journalctl -u proxwarden -f --since "1 hour ago"
```

## Development

### Architecture Overview

ProxWarden follows a modular architecture designed for maintainability and extensibility:

```
internal/
├── api/        # Proxmox API client and operations
├── config/     # Configuration management and validation  
├── health/     # Health checking implementations (TCP, HTTP, ICMP)
├── failover/   # Backup-restore failover orchestration
├── monitor/    # Container state tracking and monitoring
└── daemon/     # Systemd service implementation

cmd/proxwarden/ # CLI commands and interfaces
```

### Building from Source

```bash
# Download dependencies
make deps

# Build for current platform  
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run all quality checks
make check
```

### Development Workflow

1. **Setup Development Environment**:
   ```bash
   git clone https://github.com/jbutlerdev/proxwarden.git
   cd proxwarden
   go mod download
   ```

2. **Run in Development Mode**:
   ```bash
   make dev  # Builds and runs with debug logging
   ```

3. **Run Tests**:
   ```bash
   make test           # Unit tests
   make test-race      # Race condition detection
   make test-coverage  # Generate coverage report
   ```

4. **Code Quality**:
   ```bash
   make fmt   # Format code
   make lint  # Run linter
   make vet   # Run go vet
   make check # All quality checks
   ```

### Key Design Patterns

- **Interface-based Design**: Core components use interfaces for testability
- **Context Propagation**: All operations support cancellation via context
- **Structured Logging**: Uses logrus with consistent field naming
- **Error Wrapping**: Errors include context using `fmt.Errorf("context: %w", err)`
- **Configuration Validation**: Fail-fast approach with comprehensive validation

### Testing Strategy

ProxWarden uses comprehensive testing:

- **Unit Tests**: Individual component testing with mocks
- **Integration Tests**: API interaction testing
- **Health Check Tests**: Network operation testing with timeouts
- **Configuration Tests**: YAML parsing and validation testing

### Adding New Features

1. **Health Check Types**: Implement in `internal/health/checker.go`
2. **CLI Commands**: Add to `cmd/proxwarden/`
3. **API Operations**: Extend `internal/api/client.go`
4. **Configuration Options**: Update `internal/config/config.go`

### Documentation Files

- `CLAUDE.md` - AI assistant development guide
- `llms.txt` - LLM context for code understanding
- `README.md` - User and developer documentation

### Debugging Tips

1. **Enable Verbose Logging**:
   ```bash
   proxwarden daemon --debug --log-level debug
   ```

2. **Test Individual Components**:
   ```bash
   proxwarden status           # Test API connectivity
   proxwarden failover trigger # Test failover logic
   proxwarden backup list      # Test backup operations
   ```

3. **Monitor Service Logs**:
   ```bash
   journalctl -u proxwarden -f
   ```

### Performance Considerations

- Health checks run concurrently per container
- Configurable monitoring intervals to balance load vs responsiveness  
- Backup/restore operations have timeout controls
- State management uses appropriate synchronization

### Extension Points

The architecture supports extension in several areas:

- **Custom Health Checks**: Add new types in `health` package
- **Storage Backends**: Extend backup operations in `api` package  
- **Notification Systems**: Add hooks in `failover` package
- **Web Interface**: Create new packages in `pkg/` directory
- **Metrics Integration**: Extend `monitor` package

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the established patterns
4. Add tests for new functionality
5. Run `make check` to ensure code quality
6. Update relevant documentation
7. Submit a pull request with clear description

### Code Style Guidelines

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting
- Include comprehensive error handling
- Add structured logging for debugging
- Write table-driven tests for multiple scenarios
- Document public APIs with clear comments

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

- Issues: [GitHub Issues](https://github.com/jbutlerdev/proxwarden/issues)
- Documentation: [Wiki](https://github.com/jbutlerdev/proxwarden/wiki)
- Discussions: [GitHub Discussions](https://github.com/jbutlerdev/proxwarden/discussions)