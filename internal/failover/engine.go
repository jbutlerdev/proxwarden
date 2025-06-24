package failover

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	config    *config.Config
	apiClient *api.Client
	logger    *logrus.Logger
}

type FailoverResult struct {
	ContainerID   int
	SourceNode    string
	TargetNode    string
	Success       bool
	Error         error
	Duration      time.Duration
	StartTime     time.Time
	EndTime       time.Time
}

func New(logger *logrus.Logger) (*Engine, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	apiClient, err := api.NewClient(&cfg.Proxmox)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Engine{
		config:    cfg,
		apiClient: apiClient,
		logger:    logger,
	}, nil
}

func NewWithConfig(cfg *config.Config, apiClient *api.Client, logger *logrus.Logger) *Engine {
	return &Engine{
		config:    cfg,
		apiClient: apiClient,
		logger:    logger,
	}
}

func (e *Engine) TriggerFailover(containerID int, targetNode string, force bool) error {
	ctx := context.Background()
	
	// Find container config
	var containerConfig *config.ContainerConfig
	for _, c := range e.config.Monitoring.Containers {
		if c.ID == containerID {
			containerConfig = &c
			break
		}
	}
	
	if containerConfig == nil {
		return fmt.Errorf("container %d not found in configuration", containerID)
	}

	// Get current container info
	containerInfo, err := e.apiClient.GetContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	// Check if force is required
	if !force && containerInfo.Status == "running" {
		return fmt.Errorf("container is running and force flag not set")
	}

	// Determine target node
	if targetNode == "" {
		targetNode, err = e.selectBestNode(ctx, containerConfig, containerInfo.Node)
		if err != nil {
			return fmt.Errorf("failed to select target node: %w", err)
		}
	}

	e.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"source_node":  containerInfo.Node,
		"target_node":  targetNode,
		"force":        force,
	}).Info("Starting manual failover")

	result := e.performFailover(ctx, containerConfig, containerInfo.Node, targetNode)
	
	if result.Success {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerID,
			"source_node":  result.SourceNode,
			"target_node":  result.TargetNode,
			"duration":     result.Duration,
		}).Info("Manual failover completed successfully")
	} else {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerID,
			"error":        result.Error,
		}).Error("Manual failover failed")
		return result.Error
	}

	return nil
}

func (e *Engine) HandleContainerFailure(containerID int) error {
	if !e.config.Failover.AutoFailover {
		e.logger.WithField("container_id", containerID).Info("Auto-failover disabled, skipping")
		return nil
	}

	ctx := context.Background()
	
	// Find container config
	var containerConfig *config.ContainerConfig
	for _, c := range e.config.Monitoring.Containers {
		if c.ID == containerID {
			containerConfig = &c
			break
		}
	}
	
	if containerConfig == nil {
		return fmt.Errorf("container %d not found in configuration", containerID)
	}

	// Get current container info
	containerInfo, err := e.apiClient.GetContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	// Select best target node
	targetNode, err := e.selectBestNode(ctx, containerConfig, containerInfo.Node)
	if err != nil {
		return fmt.Errorf("failed to select target node: %w", err)
	}

	e.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"source_node":  containerInfo.Node,
		"target_node":  targetNode,
	}).Info("Starting automatic failover")

	result := e.performFailover(ctx, containerConfig, containerInfo.Node, targetNode)
	
	if result.Success {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerID,
			"source_node":  result.SourceNode,
			"target_node":  result.TargetNode,
			"duration":     result.Duration,
		}).Info("Automatic failover completed successfully")
	} else {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerID,
			"error":        result.Error,
		}).Error("Automatic failover failed")
		return result.Error
	}

	return nil
}

func (e *Engine) selectBestNode(ctx context.Context, containerConfig *config.ContainerConfig, currentNode string) (string, error) {
	if len(containerConfig.FailoverNodes) == 0 {
		return "", fmt.Errorf("no failover nodes configured for container %d", containerConfig.ID)
	}

	// Get all nodes status
	nodes, err := e.apiClient.GetNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}

	nodeStatus := make(map[string]*api.NodeInfo)
	for _, node := range nodes {
		nodeStatus[node.Name] = node
	}

	// Filter available failover nodes
	type nodeCandidate struct {
		name     string
		priority int
		online   bool
	}

	var candidates []nodeCandidate
	for i, nodeName := range containerConfig.FailoverNodes {
		if nodeName == currentNode {
			continue // Skip current node
		}

		node, exists := nodeStatus[nodeName]
		if !exists {
			e.logger.WithField("node", nodeName).Warn("Configured failover node not found in cluster")
			continue
		}

		candidates = append(candidates, nodeCandidate{
			name:     nodeName,
			priority: i, // Lower index = higher priority
			online:   node.Online,
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no available failover nodes for container %d", containerConfig.ID)
	}

	// Sort by online status first, then by priority
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].online != candidates[j].online {
			return candidates[i].online // Online nodes first
		}
		return candidates[i].priority < candidates[j].priority // Lower priority index first
	})

	if !candidates[0].online {
		return "", fmt.Errorf("no online failover nodes available for container %d", containerConfig.ID)
	}

	return candidates[0].name, nil
}

func (e *Engine) performFailover(ctx context.Context, containerConfig *config.ContainerConfig, sourceNode, targetNode string) *FailoverResult {
	result := &FailoverResult{
		ContainerID: containerConfig.ID,
		SourceNode:  sourceNode,
		TargetNode:  targetNode,
		StartTime:   time.Now(),
	}

	// Execute pre-failover hooks
	if err := e.executeHooks(e.config.Failover.PreFailoverHooks, containerConfig); err != nil {
		result.Error = fmt.Errorf("pre-failover hooks failed: %w", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	var err error
	var backupPath string

	// Step 1: Create backup if required or find latest backup
	if e.config.Failover.BackupBeforeFailover {
		e.logger.WithField("container_id", containerConfig.ID).Info("Creating backup before failover")
		
		backupStorage := containerConfig.BackupStorage
		if backupStorage == "" {
			backupStorage = e.config.Backup.Storage
		}

		backupPath, err = e.apiClient.BackupContainer(ctx, containerConfig.ID, backupStorage, e.config.Backup.BackupDir)
		if err != nil {
			result.Error = fmt.Errorf("backup failed: %w", err)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}

		e.logger.WithFields(logrus.Fields{
			"container_id": containerConfig.ID,
			"backup_path":  backupPath,
		}).Info("Backup completed successfully")
	} else {
		// Find the latest backup
		backupPath, err = e.findLatestBackup(ctx, containerConfig.ID)
		if err != nil {
			result.Error = fmt.Errorf("failed to find backup: %w", err)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}
	}

	// Perform backup-restore based failover with retries
	for attempt := 1; attempt <= e.config.Failover.MaxRetries; attempt++ {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerConfig.ID,
			"attempt":      attempt,
			"max_retries":  e.config.Failover.MaxRetries,
		}).Info("Attempting backup-restore failover")

		err = e.performBackupRestoreFailover(ctx, containerConfig, sourceNode, targetNode, backupPath)
		if err == nil {
			result.Success = true
			break
		}

		e.logger.WithFields(logrus.Fields{
			"container_id": containerConfig.ID,
			"attempt":      attempt,
			"error":        err,
		}).Warn("Backup-restore failover attempt failed")

		if attempt < e.config.Failover.MaxRetries {
			time.Sleep(e.config.Failover.RetryDelay)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Error = fmt.Errorf("backup-restore failover failed after %d attempts: %w", e.config.Failover.MaxRetries, err)
		return result
	}

	// Execute post-failover hooks
	if err := e.executeHooks(e.config.Failover.PostFailoverHooks, containerConfig); err != nil {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerConfig.ID,
			"error":        err,
		}).Warn("Post-failover hooks failed, but failover was successful")
	}

	return result
}

func (e *Engine) performBackupRestoreFailover(ctx context.Context, containerConfig *config.ContainerConfig, sourceNode, targetNode, backupPath string) error {
	containerID := containerConfig.ID

	// Step 1: Stop original container if reachable
	e.logger.WithField("container_id", containerID).Info("Attempting to stop original container")
	if err := e.apiClient.StopContainer(ctx, containerID); err != nil {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerID,
			"error":        err,
		}).Warn("Failed to stop original container, continuing with restore")
	} else {
		e.logger.WithField("container_id", containerID).Info("Original container stopped successfully")
	}

	// Step 2: Restore from backup on target node
	e.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"target_node":  targetNode,
		"backup_path":  backupPath,
	}).Info("Restoring container from backup")

	storage := containerConfig.Storage
	if storage == "" {
		storage = "local-lvm" // Default storage
	}

	err := e.apiClient.RestoreContainerFromBackup(ctx, containerID, targetNode, storage, backupPath, true)
	if err != nil {
		return fmt.Errorf("failed to restore container: %w", err)
	}

	// Step 3: Start the restored container
	e.logger.WithField("container_id", containerID).Info("Starting restored container")
	err = e.apiClient.StartContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to start restored container: %w", err)
	}

	e.logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"target_node":  targetNode,
	}).Info("Container successfully restored and started on target node")

	return nil
}

func (e *Engine) findLatestBackup(ctx context.Context, containerID int) (string, error) {
	backups, err := e.apiClient.GetBackups(ctx, e.config.Backup.Storage)
	if err != nil {
		return "", fmt.Errorf("failed to get backups: %w", err)
	}

	var latestBackup *api.BackupInfo
	containerBackupPrefix := fmt.Sprintf("vzdump-lxc-%d-", containerID)

	for _, backup := range backups {
		if !strings.HasPrefix(backup.Filename, containerBackupPrefix) {
			continue
		}

		if latestBackup == nil || backup.Filename > latestBackup.Filename {
			backupCopy := backup
			latestBackup = &backupCopy
		}
	}

	if latestBackup == nil {
		return "", fmt.Errorf("no backup found for container %d", containerID)
	}

	return fmt.Sprintf("%s:%s", latestBackup.Storage, latestBackup.Filename), nil
}

func (e *Engine) executeHooks(hooks []string, containerConfig *config.ContainerConfig) error {
	if len(hooks) == 0 {
		return nil
	}

	for _, hook := range hooks {
		e.logger.WithFields(logrus.Fields{
			"container_id": containerConfig.ID,
			"hook":         hook,
		}).Info("Executing hook")

		cmd := exec.Command("sh", "-c", hook)
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("CONTAINER_ID=%d", containerConfig.ID),
			fmt.Sprintf("CONTAINER_NAME=%s", containerConfig.Name),
		)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook '%s' failed: %w", hook, err)
		}
	}

	return nil
}