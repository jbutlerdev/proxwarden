package proxwarden

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup management operations",
	Long:  `Manage container backups used for failover operations.`,
}

var backupCreateCmd = &cobra.Command{
	Use:   "create [container-id]",
	Short: "Create a backup of a container",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupCreate,
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backups",
	RunE:  runBackupList,
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [container-id] [backup-path]",
	Short: "Restore container from backup",
	Args:  cobra.ExactArgs(2),
	RunE:  runBackupRestore,
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupRestoreCmd)

	backupCreateCmd.Flags().String("storage", "", "backup storage (uses config default)")
	
	backupListCmd.Flags().String("storage", "", "storage to list backups from")
	backupListCmd.Flags().Bool("json", false, "output in JSON format")
	
	backupRestoreCmd.Flags().String("target-node", "", "target node for restore")
	backupRestoreCmd.Flags().String("storage", "", "storage for restored container")
	backupRestoreCmd.Flags().Bool("force", false, "force restore (overwrite existing)")
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := logrus.New()
	
	containerID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid container ID: %w", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create API client
	apiClient, err := api.NewClient(&cfg.Proxmox)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	storage, _ := cmd.Flags().GetString("storage")
	if storage == "" {
		storage = cfg.Backup.Storage
	}

	logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"storage":      storage,
	}).Info("Creating backup")

	backupPath, err := apiClient.BackupContainer(ctx, containerID, storage, cfg.Backup.BackupDir)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	fmt.Printf("Backup created successfully: %s\n", backupPath)
	return nil
}

func runBackupList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create API client
	apiClient, err := api.NewClient(&cfg.Proxmox)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	storage, _ := cmd.Flags().GetString("storage")
	if storage == "" {
		storage = cfg.Backup.Storage
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")

	backups, err := apiClient.GetBackups(ctx, storage)
	if err != nil {
		return fmt.Errorf("failed to get backups: %w", err)
	}

	if jsonOutput {
		output, err := json.MarshalIndent(backups, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tSTORAGE\tFILENAME\tSIZE\tFORMAT")
	fmt.Fprintln(w, "----\t-------\t--------\t----\t------")

	for _, backup := range backups {
		sizeStr := fmt.Sprintf("%.2f MB", float64(backup.Size)/(1024*1024))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			backup.Node, backup.Storage, backup.Filename, sizeStr, backup.Format)
	}

	return w.Flush()
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := logrus.New()
	
	containerID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid container ID: %w", err)
	}

	backupPath := args[1]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create API client
	apiClient, err := api.NewClient(&cfg.Proxmox)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	targetNode, _ := cmd.Flags().GetString("target-node")
	if targetNode == "" {
		// Get current node or default to first available
		nodes, err := apiClient.GetNodes(ctx)
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		if len(nodes) == 0 {
			return fmt.Errorf("no nodes available")
		}
		targetNode = nodes[0].Name
	}

	storage, _ := cmd.Flags().GetString("storage")
	if storage == "" {
		storage = "local-lvm" // Default storage
	}

	force, _ := cmd.Flags().GetBool("force")

	logger.WithFields(logrus.Fields{
		"container_id": containerID,
		"backup_path":  backupPath,
		"target_node":  targetNode,
		"storage":      storage,
		"force":        force,
	}).Info("Restoring from backup")

	err = apiClient.RestoreContainerFromBackup(ctx, containerID, targetNode, storage, backupPath, force)
	if err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Printf("Container %d restored successfully on node %s\n", containerID, targetNode)
	return nil
}