package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	log "github.com/jaxxstorm/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger   *log.Logger
	logLevel string

	rootCmd = &cobra.Command{
		Use:           "thresher",
		Short:         "Decode Tailscale debug captures",
		Long:          "thresher is a CLI for decoding Tailscale debug captures and related packet metadata.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if logger != nil {
				_ = logger.Close()
			}

			configuredLogger, err := log.New(log.Config{Level: log.Level(logLevel)})
			if err != nil {
				return fmt.Errorf("initializing logger: %w", err)
			}

			logger = configuredLogger
			logger.Debug("logger initialized", log.String("level", logLevel))

			return nil
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error, fatal")
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	viper.SetEnvPrefix("thresher")
	viper.AutomaticEnv()
	viper.SetConfigType("yaml")
	viper.SetConfigName("thresher")
	viper.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
		viper.AddConfigPath(filepath.Join(home, ".config", "thresher"))
	}
	_ = viper.ReadInConfig()
}

func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return ExecuteContext(ctx)
}

func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}
