package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	log "github.com/jaxxstorm/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger   *log.Logger
	logLevel string

	bannerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	rootCmd = &cobra.Command{
		Use:           "thresher",
		Short:         "Decode Tailscale debug captures",
		Long:          "thresher is a CLI for decoding Tailscale debug captures and related packet metadata.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), bannerStyle.Render("hello world"))
			return err
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
	// Config file discovery will be added in a later change.
}

func Execute() error {
	return rootCmd.Execute()
}
