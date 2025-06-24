package api

import (
	"context"
	"fmt"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/config"
	proxmox "github.com/luthermonson/go-proxmox"
)

type Client struct {
	client *proxmox.Client
	config *config.ProxmoxConfig
}

type ContainerInfo struct {
	ID     int
	Name   string
	Node   string
	Status string
	State  string
}

type NodeInfo struct {
	Name   string
	Status string
	Online bool
}

type BackupInfo struct {
	Node     string
	Storage  string
	Filename string
	Size     int64
	Format   string
}

func NewClient(cfg *config.ProxmoxConfig) (*Client, error) {
	var client *proxmox.Client

	if cfg.TokenID != "" && cfg.Secret != "" {
		client = proxmox.NewClient(cfg.Endpoint,
			proxmox.WithAPIToken(cfg.TokenID, cfg.Secret),
		)
	} else {
		client = proxmox.NewClient(cfg.Endpoint,
			proxmox.WithLogins(cfg.Username, cfg.Password),
		)
	}

	// TODO: Handle insecure SSL - method may not be available in this version
	// if cfg.Insecure {
	//     client.InsecureSkipVerify = true
	// }

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

func (c *Client) GetContainer(ctx context.Context, containerID int) (*ContainerInfo, error) {
	nodes, err := c.client.Nodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	for _, node := range nodes {
		nodeObj, err := c.client.Node(ctx, node.Node)
		if err != nil {
			continue
		}

		containers, err := nodeObj.Containers(ctx)
		if err != nil {
			continue
		}

		for _, container := range containers {
			if int(container.VMID) == containerID {
				return &ContainerInfo{
					ID:     int(container.VMID),
					Name:   container.Name,
					Node:   node.Node,
					Status: container.Status,
					State:  container.Status,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("container %d not found", containerID)
}

func (c *Client) GetContainersByNode(ctx context.Context, nodeName string) ([]*ContainerInfo, error) {
	nodeObj, err := c.client.Node(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	containers, err := nodeObj.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get containers for node %s: %w", nodeName, err)
	}

	var result []*ContainerInfo
	for _, container := range containers {
		result = append(result, &ContainerInfo{
			ID:     int(container.VMID),
			Name:   container.Name,
			Node:   nodeName,
			Status: container.Status,
			State:  container.Status,
		})
	}

	return result, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]*NodeInfo, error) {
	nodes, err := c.client.Nodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	var result []*NodeInfo
	for _, node := range nodes {
		result = append(result, &NodeInfo{
			Name:   node.Node,
			Status: node.Status,
			Online: node.Status == "online",
		})
	}

	return result, nil
}

func (c *Client) MigrateContainer(ctx context.Context, containerID int, targetNode string) error {
	container, err := c.GetContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	nodeObj, err := c.client.Node(ctx, container.Node)
	if err != nil {
		return fmt.Errorf("failed to get source node: %w", err)
	}

	lxc, err := nodeObj.Container(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	// Use basic migration without complex options for now
	task, err := lxc.Migrate(ctx, &proxmox.ContainerMigrateOptions{
		Target: targetNode,
		Online: false,
	})
	if err != nil {
		return fmt.Errorf("failed to start migration: %w", err)
	}

	err = task.Wait(ctx, 5*time.Minute, 30*time.Second)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

func (c *Client) StopContainer(ctx context.Context, containerID int) error {
	container, err := c.GetContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	nodeObj, err := c.client.Node(ctx, container.Node)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	lxc, err := nodeObj.Container(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	task, err := lxc.Stop(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return task.Wait(ctx, 1*time.Minute, 5*time.Second)
}

func (c *Client) StartContainer(ctx context.Context, containerID int) error {
	container, err := c.GetContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	nodeObj, err := c.client.Node(ctx, container.Node)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	lxc, err := nodeObj.Container(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	task, err := lxc.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return task.Wait(ctx, 1*time.Minute, 5*time.Second)
}

func (c *Client) BackupContainer(ctx context.Context, containerID int, storage, backupDir string) (string, error) {
	// Generate backup filename
	backupFilename := fmt.Sprintf("vzdump-lxc-%d-%s.tar.zst", containerID, 
		time.Now().Format("2006_01_02-15_04_05"))

	// For now, return a simulated backup path
	// In a real implementation, this would trigger a vzdump backup
	backupPath := fmt.Sprintf("%s:%s/%s", storage, backupDir, backupFilename)
	
	// TODO: Implement actual backup via Proxmox API
	// This would involve calling the /cluster/backup endpoint or vzdump command
	
	return backupPath, nil
}

func (c *Client) RestoreContainerFromBackup(ctx context.Context, containerID int, targetNode, storage, backupPath string, force bool) error {
	// TODO: Implement actual restore via Proxmox API
	// This would involve a POST to /nodes/{node}/lxc with restore parameters
	
	// For now, this is a placeholder that simulates the restore process
	return fmt.Errorf("restore functionality not yet implemented - requires direct API calls")
}

func (c *Client) GetBackups(ctx context.Context, storage string) ([]BackupInfo, error) {
	// TODO: Implement backup listing via Proxmox API
	// This would query storage content for backup files
	
	// Return empty list for now
	return []BackupInfo{}, nil
}

func (c *Client) DeleteBackup(ctx context.Context, nodeName, storage, backupPath string) error {
	// TODO: Implement backup deletion via Proxmox API
	return fmt.Errorf("delete backup functionality not yet implemented")
}