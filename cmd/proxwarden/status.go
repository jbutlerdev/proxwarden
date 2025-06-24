package proxwarden

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/api"
	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of monitored containers",
	Long:  `Display the current status and health of all monitored containers.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("json", false, "output in JSON format")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise for status command

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

	jsonOutput, _ := cmd.Flags().GetBool("json")

	type ContainerStatus struct {
		ID           int       `json:"id"`
		Name         string    `json:"name"`
		Node         string    `json:"node"`
		Status       string    `json:"status"`
		LastChecked  time.Time `json:"last_checked,omitempty"`
		HealthStatus string    `json:"health_status"`
		Error        string    `json:"error,omitempty"`
	}

	var containerStatuses []ContainerStatus

	for _, container := range cfg.Monitoring.Containers {
		status := ContainerStatus{
			ID:           container.ID,
			Name:         container.Name,
			LastChecked:  time.Now(),
			HealthStatus: "unknown",
		}

		// Get container info from Proxmox
		containerInfo, err := apiClient.GetContainer(ctx, container.ID)
		if err != nil {
			status.Error = err.Error()
			status.HealthStatus = "error"
		} else {
			status.Node = containerInfo.Node
			status.Status = containerInfo.Status
			status.HealthStatus = "reachable"
		}

		containerStatuses = append(containerStatuses, status)
	}

	if jsonOutput {
		output, err := json.MarshalIndent(containerStatuses, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Text output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tNODE\tSTATUS\tHEALTH\tERROR")
	fmt.Fprintln(w, "--\t----\t----\t------\t------\t-----")

	for _, status := range containerStatuses {
		errorStr := status.Error
		if len(errorStr) > 50 {
			errorStr = errorStr[:47] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			status.ID, status.Name, status.Node, status.Status, status.HealthStatus, errorStr)
	}

	return w.Flush()
}