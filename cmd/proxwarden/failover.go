package proxwarden

import (
	"strconv"

	"github.com/jbutlerdev/proxwarden/internal/failover"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var failoverCmd = &cobra.Command{
	Use:   "failover",
	Short: "Manual failover operations",
	Long:  `Manually trigger failover operations for containers.`,
}

var triggerCmd = &cobra.Command{
	Use:   "trigger [container-id]",
	Short: "Trigger manual failover for a container",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrigger,
}

func init() {
	rootCmd.AddCommand(failoverCmd)
	failoverCmd.AddCommand(triggerCmd)
	
	triggerCmd.Flags().String("target-node", "", "target node for failover (optional)")
	triggerCmd.Flags().Bool("force", false, "force failover even if container is healthy")
}

func runTrigger(cmd *cobra.Command, args []string) error {
	logger := logrus.New()
	
	containerID, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	targetNode, _ := cmd.Flags().GetString("target-node")
	force, _ := cmd.Flags().GetBool("force")

	engine, err := failover.New(logger)
	if err != nil {
		return err
	}

	return engine.TriggerFailover(containerID, targetNode, force)
}