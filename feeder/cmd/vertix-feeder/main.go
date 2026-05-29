package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cosmossdk.io/log"
	"github.com/spf13/cobra"

	"github.com/vertix-network/vertix/feeder/feeder"

	// Seal the SDK Bech32 config (vtx/vtxvaloper) on import.
	_ "github.com/vertix-network/vertix/app"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var configPath string
	var logLevel string
	cmd := &cobra.Command{
		Use:   "vertix-feeder",
		Short: "Vertix oracle price-feed sidecar",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := feeder.LoadConfig(configPath)
			if err != nil {
				return err
			}
			if logLevel == "" {
				logLevel = cfg.LogLevel
			}
			var opts []log.Option
			if filter, err := log.ParseLogLevel(logLevel); err == nil {
				opts = append(opts, log.FilterOption(filter))
			}
			logger := log.NewLogger(os.Stderr, opts...)
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			f, err := feeder.New(cfg, logger)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			logger.Info("vertix-feeder starting", "chain_id", cfg.ChainID, "node", cfg.NodeGRPC, "pairs", len(cfg.Pairs))
			f.Run(ctx)
			logger.Info("vertix-feeder stopped")
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "feeder-config.yaml", "path to the feeder config YAML")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "log level (overrides config)")
	return cmd
}
