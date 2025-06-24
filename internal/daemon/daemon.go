package daemon

import (
	"context"
	"fmt"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/jbutlerdev/proxwarden/internal/failover"
	"github.com/jbutlerdev/proxwarden/internal/monitor"
	"github.com/sirupsen/logrus"
)

type Daemon struct {
	config        *config.Config
	apiClient     *api.Client
	monitor       *monitor.Monitor
	failoverEngine *failover.Engine
	logger        *logrus.Logger
}

func New(logger *logrus.Logger) (*Daemon, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup logging
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	// Create API client
	apiClient, err := api.NewClient(&cfg.Proxmox)
	if err != nil {
		return nil, fmt.Errorf("failed to create Proxmox API client: %w", err)
	}

	// Create failover engine
	failoverEngine := failover.NewWithConfig(cfg, apiClient, logger)

	// Create monitor
	monitorService := monitor.New(cfg, apiClient, logger)

	// Setup failover callback
	monitorService.AddFailureCallback(func(containerID int, state *monitor.ContainerState) {
		logger.WithFields(logrus.Fields{
			"container_id":  containerID,
			"failure_count": state.FailureCount,
		}).Warn("Container failure detected, initiating failover")

		if err := failoverEngine.HandleContainerFailure(containerID); err != nil {
			logger.WithFields(logrus.Fields{
				"container_id": containerID,
				"error":        err,
			}).Error("Failover failed")
		}
	})

	return &Daemon{
		config:         cfg,
		apiClient:      apiClient,
		monitor:        monitorService,
		failoverEngine: failoverEngine,
		logger:         logger,
	}, nil
}

func (d *Daemon) Start(ctx context.Context) error {
	d.logger.Info("Starting ProxWarden daemon")

	// Validate Proxmox connectivity
	if err := d.validateConnectivity(ctx); err != nil {
		return fmt.Errorf("failed to validate Proxmox connectivity: %w", err)
	}

	// Start monitoring
	return d.monitor.Start(ctx)
}

func (d *Daemon) validateConnectivity(ctx context.Context) error {
	d.logger.Info("Validating Proxmox connectivity")

	nodes, err := d.apiClient.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	d.logger.WithField("node_count", len(nodes)).Info("Successfully connected to Proxmox cluster")

	// Validate configured containers exist
	for _, container := range d.config.Monitoring.Containers {
		containerInfo, err := d.apiClient.GetContainer(ctx, container.ID)
		if err != nil {
			d.logger.WithFields(logrus.Fields{
				"container_id": container.ID,
				"error":        err,
			}).Warn("Configured container not found")
			continue
		}

		d.logger.WithFields(logrus.Fields{
			"container_id": container.ID,
			"name":         containerInfo.Name,
			"node":         containerInfo.Node,
			"status":       containerInfo.Status,
		}).Info("Found configured container")
	}

	return nil
}

func (d *Daemon) GetMonitor() *monitor.Monitor {
	return d.monitor
}

func (d *Daemon) GetFailoverEngine() *failover.Engine {
	return d.failoverEngine
}