package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Proxmox   ProxmoxConfig   `yaml:"proxmox"`
	Backup    BackupConfig    `yaml:"backup"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Failover  FailoverConfig  `yaml:"failover"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type ProxmoxConfig struct {
	Endpoint string `yaml:"endpoint"`
	Username string `yaml:"username"`
	Password string `yaml:"password,omitempty"`
	TokenID  string `yaml:"token_id,omitempty"`
	Secret   string `yaml:"secret,omitempty"`
	Insecure bool   `yaml:"insecure"`
}

type BackupConfig struct {
	Storage       string        `yaml:"storage"`
	BackupDir     string        `yaml:"backup_dir"`
	RetentionDays int           `yaml:"retention_days"`
	PreBackup     bool          `yaml:"pre_backup"`
	BackupTimeout time.Duration `yaml:"backup_timeout"`
}

type MonitoringConfig struct {
	Interval        time.Duration `yaml:"interval"`
	Timeout         time.Duration `yaml:"timeout"`
	FailureThreshold int          `yaml:"failure_threshold"`
	Containers      []ContainerConfig `yaml:"containers"`
}

type ContainerConfig struct {
	ID           int      `yaml:"id"`
	Name         string   `yaml:"name"`
	HealthChecks []HealthCheck `yaml:"health_checks"`
	Priority     int      `yaml:"priority"`
	FailoverNodes []string `yaml:"failover_nodes"`
	Storage      string   `yaml:"storage"`
	BackupStorage string  `yaml:"backup_storage,omitempty"`
}

type HealthCheck struct {
	Type     string        `yaml:"type"`
	Target   string        `yaml:"target"`
	Port     int           `yaml:"port,omitempty"`
	Path     string        `yaml:"path,omitempty"`
	Timeout  time.Duration `yaml:"timeout"`
	Interval time.Duration `yaml:"interval"`
}

type FailoverConfig struct {
	AutoFailover     bool          `yaml:"auto_failover"`
	MaxRetries       int           `yaml:"max_retries"`
	RetryDelay       time.Duration `yaml:"retry_delay"`
	BackupBeforeFailover bool       `yaml:"backup_before_failover"`
	RestoreTimeout   time.Duration `yaml:"restore_timeout"`
	PreFailoverHooks []string      `yaml:"pre_failover_hooks"`
	PostFailoverHooks []string     `yaml:"post_failover_hooks"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file,omitempty"`
}

func Load() (*Config, error) {
	config := &Config{
		Proxmox: ProxmoxConfig{
			Insecure: false,
		},
		Backup: BackupConfig{
			Storage:       "local",
			BackupDir:     "backup",
			RetentionDays: 7,
			PreBackup:     true,
			BackupTimeout: 10 * time.Minute,
		},
		Monitoring: MonitoringConfig{
			Interval:        30 * time.Second,
			Timeout:         10 * time.Second,
			FailureThreshold: 3,
		},
		Failover: FailoverConfig{
			AutoFailover:         true,
			MaxRetries:           3,
			RetryDelay:           5 * time.Second,
			BackupBeforeFailover: true,
			RestoreTimeout:       15 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func validate(config *Config) error {
	if config.Proxmox.Endpoint == "" {
		return fmt.Errorf("proxmox endpoint is required")
	}

	if config.Proxmox.Username == "" {
		return fmt.Errorf("proxmox username is required")
	}

	if config.Proxmox.Password == "" && (config.Proxmox.TokenID == "" || config.Proxmox.Secret == "") {
		return fmt.Errorf("either password or token authentication must be configured")
	}

	if len(config.Monitoring.Containers) == 0 {
		return fmt.Errorf("at least one container must be configured for monitoring")
	}

	for _, container := range config.Monitoring.Containers {
		if container.ID <= 0 {
			return fmt.Errorf("container ID must be positive")
		}
		if len(container.HealthChecks) == 0 {
			return fmt.Errorf("container %d must have at least one health check", container.ID)
		}
		if len(container.FailoverNodes) == 0 {
			return fmt.Errorf("container %d must have at least one failover node", container.ID)
		}
	}

	return nil
}

func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}