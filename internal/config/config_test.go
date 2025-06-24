package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	configContent := `
proxmox:
  endpoint: "https://test-server:8006"
  username: "root@pam"
  password: "testpass"
  insecure: true

backup:
  storage: "local"
  backup_dir: "backup"
  retention_days: 7
  pre_backup: true
  backup_timeout: 10m

monitoring:
  interval: 30s
  timeout: 10s
  failure_threshold: 3
  containers:
    - id: 100
      name: "test-container"
      priority: 1
      storage: "local-lvm"
      failover_nodes: ["node2", "node3"]
      health_checks:
        - type: "tcp"
          target: "192.168.1.100"
          port: 80
          timeout: 5s
          interval: 30s

failover:
  auto_failover: true
  max_retries: 3
  retry_delay: 5s
  backup_before_failover: true
  restore_timeout: 15m

logging:
  level: "info"
  format: "json"
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "proxwarden-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set config file for viper
	viper.Reset()
	viper.SetConfigFile(tmpFile.Name())
	
	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test proxmox config
	if config.Proxmox.Endpoint != "https://test-server:8006" {
		t.Errorf("Expected endpoint 'https://test-server:8006', got '%s'", config.Proxmox.Endpoint)
	}
	if config.Proxmox.Username != "root@pam" {
		t.Errorf("Expected username 'root@pam', got '%s'", config.Proxmox.Username)
	}
	if !config.Proxmox.Insecure {
		t.Error("Expected insecure to be true")
	}

	// Test monitoring config
	if config.Monitoring.Interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", config.Monitoring.Interval)
	}
	if config.Monitoring.FailureThreshold != 3 {
		t.Errorf("Expected failure threshold 3, got %d", config.Monitoring.FailureThreshold)
	}
	if len(config.Monitoring.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(config.Monitoring.Containers))
	}

	container := config.Monitoring.Containers[0]
	if container.ID != 100 {
		t.Errorf("Expected container ID 100, got %d", container.ID)
	}
	if len(container.HealthChecks) != 1 {
		t.Errorf("Expected 1 health check, got %d", len(container.HealthChecks))
	}

	// Test failover config
	if !config.Failover.AutoFailover {
		t.Error("Expected auto_failover to be true")
	}
	if config.Failover.MaxRetries != 3 {
		t.Errorf("Expected max_retries 3, got %d", config.Failover.MaxRetries)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Endpoint: "https://test:8006",
					Username: "root@pam",
					Password: "pass",
				},
				Monitoring: MonitoringConfig{
					Containers: []ContainerConfig{
						{
							ID:   100,
							Name: "test",
							HealthChecks: []HealthCheck{
								{Type: "tcp", Target: "1.1.1.1", Port: 80},
							},
							FailoverNodes: []string{"node2"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing endpoint",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Username: "root@pam",
					Password: "pass",
				},
			},
			expectError: true,
		},
		{
			name: "missing authentication",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Endpoint: "https://test:8006",
					Username: "root@pam",
				},
			},
			expectError: true,
		},
		{
			name: "no containers",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Endpoint: "https://test:8006",
					Username: "root@pam",
					Password: "pass",
				},
				Monitoring: MonitoringConfig{
					Containers: []ContainerConfig{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}