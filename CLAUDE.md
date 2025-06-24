# Claude Development Guide for ProxWarden

This file contains important information for Claude (AI assistant) when working on the ProxWarden project.

## Project Overview

ProxWarden is a Golang CLI tool and systemd service for automating failover of critical Proxmox containers using backup/restore methodology rather than live migration.

## Architecture

```
ProxWarden/
├── cmd/proxwarden/          # CLI commands and main entry points
├── internal/                # Private application code
│   ├── api/                 # Proxmox API client wrapper
│   ├── config/              # Configuration management and validation
│   ├── health/              # Health checking service (TCP, HTTP, ICMP)
│   ├── failover/            # Backup-restore failover orchestration
│   ├── monitor/             # Container monitoring and state management
│   └── daemon/              # Systemd service implementation
├── pkg/                     # Public API packages (future use)
├── configs/                 # Example configurations
├── systemd/                 # Systemd service files
└── scripts/                 # Installation and utility scripts
```

## Key Technologies

- **Language**: Go 1.21+
- **CLI Framework**: Cobra
- **Configuration**: Viper (YAML)
- **Logging**: Logrus with structured logging
- **Proxmox API**: luthermonson/go-proxmox v0.1.1
- **Testing**: Standard Go testing with table-driven tests

## Core Failover Process

**IMPORTANT**: ProxWarden uses backup/restore failover, NOT live migration:

1. **Create/Find Backup**: Either create fresh backup or use latest existing backup
2. **Stop Original Container**: Attempt to gracefully stop the failed container
3. **Restore from Backup**: Restore container from backup on target node
4. **Start Restored Container**: Start the newly restored container
5. **Execute Hooks**: Run post-failover hooks (DNS updates, notifications, etc.)

## Build Commands

```bash
make build           # Build binary
make test           # Run all tests
make test-coverage  # Run tests with coverage
make install        # Install system-wide
make clean          # Clean build artifacts
make check          # Run all code quality checks
```

## Configuration Structure

Key configuration sections:
- `proxmox`: API connection settings
- `backup`: Backup storage and retention settings
- `monitoring`: Health checks and container definitions
- `failover`: Failover behavior and timeouts
- `logging`: Log levels and formatting

## API Client Interface

The API client (`internal/api/client.go`) provides these key methods:
- `GetContainer()` - Retrieve container information
- `BackupContainer()` - Create container backup
- `RestoreContainerFromBackup()` - Restore from backup
- `StopContainer()` / `StartContainer()` - Container lifecycle
- `MigrateContainer()` - Legacy migration (not used in backup-restore mode)

## Health Checking

Supports multiple health check types:
- **TCP**: Port connectivity checks
- **HTTP/HTTPS**: HTTP endpoint checks with status code validation
- **ICMP/Ping**: Network reachability (requires elevated permissions)

## Testing Guidelines

- Use table-driven tests for multiple scenarios
- Mock external dependencies (Proxmox API, network calls)
- Test both success and failure paths
- Include context timeout testing for network operations

## Important Files

- `main.go` - Application entry point
- `cmd/proxwarden/root.go` - Root CLI command setup
- `internal/config/config.go` - Configuration structure and validation
- `internal/failover/engine.go` - Core failover logic
- `internal/monitor/monitor.go` - Container monitoring loop
- `configs/proxwarden.example.yaml` - Example configuration

## Development Notes

1. **Error Handling**: Always wrap errors with context using `fmt.Errorf("context: %w", err)`
2. **Logging**: Use structured logging with fields for better debugging
3. **Contexts**: Pass contexts through all API calls for cancellation support
4. **Configuration**: Validate all config at startup, fail fast on invalid config
5. **Testing**: Mock the Proxmox API client for unit tests

## Common Issues and Solutions

### API Client Issues
- The go-proxmox library has evolving APIs - check version compatibility
- Some methods like backup/restore may need direct HTTP calls to Proxmox API
- Always handle connection timeouts and retries

### Container Management
- Container IDs are integers, not strings
- Status can be "running", "stopped", "paused", etc.
- Always check container exists before operations

### Backup/Restore Process
- Backup paths use format: `storage:backup/vzdump-lxc-{id}-{timestamp}.tar.zst`
- Restore requires target node, storage, and backup path
- Force flag may be needed to overwrite existing containers

## Extension Points

The architecture is designed for extension:
- Add new health check types in `internal/health/`
- Add new API providers by implementing the client interface
- Add web UI by creating new packages in `pkg/`
- Add metrics/monitoring by extending the monitor package

## Security Considerations

- Run as dedicated `proxwarden` user with minimal privileges
- Store sensitive config (passwords, tokens) securely
- Use API tokens instead of passwords when possible
- Validate all external inputs (config, API responses)

## Debugging Tips

1. **Enable Debug Logging**: `--debug --log-level debug`
2. **Check Service Status**: `systemctl status proxwarden`
3. **View Logs**: `journalctl -u proxwarden -f`
4. **Test Config**: `proxwarden status` to validate connection
5. **Manual Failover**: Use `proxwarden failover trigger <id>` for testing

## When Adding New Features

1. Update configuration structure if needed (`internal/config/`)
2. Add CLI commands in `cmd/proxwarden/`
3. Implement core logic in appropriate `internal/` package
4. Add unit tests with mocks
5. Update example configuration
6. Update this CLAUDE.md file with new information

Remember: ProxWarden prioritizes reliability and data safety through backup/restore over speed of live migration.