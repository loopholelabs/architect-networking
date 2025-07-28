package failover

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/loopholelabs/cmdutils"
	"github.com/loopholelabs/cmdutils/pkg/command"

	"github.com/loopholelabs/architect-networking/internal/config"
	"github.com/loopholelabs/architect-networking/pkg/failover"
)

func Cmd() command.SetupCommand[*config.Config] {
	return func(cmd *cobra.Command, ch *cmdutils.Helper[*config.Config]) {
		var leaderCfg failover.LeaderConfig

		c := &cobra.Command{
			Use:   "failover",
			Short: "Run Conduit failover daemon",
			Long:  "Run a daemon that handles failover between nodes using AWS ENI-based leader election",
			PreRunE: func(_ *cobra.Command, _ []string) error {
				return leaderCfg.Validate()
			},
			RunE: func(_ *cobra.Command, _ []string) error {
				ch.Printer.Printf("Starting Conduit failover daemon...")
				ch.Printer.Printf("ENI IP: %s", leaderCfg.ENIIP)
				ch.Printer.Printf("Port: %d", leaderCfg.Port)
				ch.Printer.Printf("Local socket: %s", leaderCfg.LocalSocket)
				ch.Printer.Printf("Destination CIDR: %s", leaderCfg.DestinationCIDR)
				ch.Printer.Printf("Leader check interval: %s", leaderCfg.LeaderCheckInterval)
				ch.Printer.Printf("Sync interval: %s", leaderCfg.SyncInterval)
				ch.Printer.Printf("Heartbeat interval: %s", leaderCfg.HeartbeatInterval)
				ch.Printer.Printf("Heartbeat miss threshold: %d", leaderCfg.HeartbeatMissThreshold)

				return runLeaderFailoverCmd(ch, &leaderCfg)
			},
		}

		// Failover configuration flags
		c.Flags().StringVar(&leaderCfg.ENIIP, "eni-ip", "", "ENI IP address to monitor for ownership (required)")
		c.Flags().Uint16Var(&leaderCfg.Port, "port", 1022, "Port for fRPC communication between nodes")
		c.Flags().StringVar(&leaderCfg.LocalSocket, "local-socket", "", "Local conduit server socket for API access (required)")
		c.Flags().StringVar(&leaderCfg.DestinationCIDR, "destination-cidr", "", "Destination CIDR block for route table updates")
		c.Flags().DurationVar(&leaderCfg.LeaderCheckInterval, "leader-check-interval", 30*time.Second, "Leader election check interval")
		c.Flags().DurationVar(&leaderCfg.SyncInterval, "sync-interval", 10*time.Second, "State sync interval when acting as secondary")
		c.Flags().DurationVar(&leaderCfg.HeartbeatInterval, "heartbeat-interval", 40*time.Millisecond, "Heartbeat interval (must be <50ms for 3 heartbeats in 150ms)")
		c.Flags().IntVar(&leaderCfg.HeartbeatMissThreshold, "heartbeat-miss-threshold", 3, "Number of missed heartbeats before failover")
		c.Flags().BoolVar(&leaderCfg.DisableENICheck, "disable-eni-check", false, "Disable ENI ownership checks for testing")
		c.Flags().StringVar(&leaderCfg.ForceRole, "force-role", "", "Force role to 'primary' or 'secondary' for testing")

		// Mark required flags
		if err := c.MarkFlagRequired("eni-ip"); err != nil {
			panic(err) // This should never happen during command setup
		}
		if err := c.MarkFlagRequired("local-socket"); err != nil {
			panic(err) // This should never happen during command setup
		}

		cmd.AddCommand(c)
	}
}

func runLeaderFailoverCmd(ch *cmdutils.Helper[*config.Config], cfg *failover.LeaderConfig) error {
	logger := ch.Logger.SubLogger("LeaderFailoverCmd")
	cfg.Logger = logger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt)

		<-done

		logger.Info().Msg("Exiting gracefully")
		cancel()
	}()

	lf, err := failover.NewLeaderFailover(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := lf.Stop(); err != nil {
			logger.Error().Err(err).Msg("Failed to stop leader failover")
		}
	}()

	return lf.Start(ctx)
}
