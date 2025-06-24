package proxwarden

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbutlerdev/proxwarden/internal/daemon"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run ProxWarden as a daemon service",
	Long:  `Start the ProxWarden daemon to continuously monitor containers and perform automatic failover.`,
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	logger := logrus.New()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, gracefully stopping...")
		cancel()
	}()

	// Initialize and start daemon
	d, err := daemon.New(logger)
	if err != nil {
		return err
	}

	return d.Start(ctx)
}