package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/jaxxstorm/thresher/internal/capture"
	"github.com/spf13/cobra"
)

var captureArgs struct {
	output string
	format string
}

var openCaptureStream = capture.OpenLocalAPIStream

func init() {
	rootCmd.AddCommand(newCaptureCommand())
}

func newCaptureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "capture",
		Short:         "Stream decoded packets from tailscaled debug capture",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCapture(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVarP(&captureArgs.output, "output", "o", "", "path to write JSONL output (default stdout)")
	cmd.Flags().StringVar(&captureArgs.format, "format", string(capture.FormatJSONL), "output format: jsonl, jsonl-compact, summary, packet-list")
	return cmd
}

func runCapture(ctx context.Context, stdout, stderr io.Writer) error {
	w, closer, err := capture.SelectOutputWriter(captureArgs.output, stdout)
	if err != nil {
		return err
	}
	defer closer.Close()

	format, err := capture.ParseOutputFormat(captureArgs.format)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(stderr, "capture started; waiting for packets (Ctrl-C to stop)"); err != nil {
		return fmt.Errorf("writing capture status: %w", err)
	}

	if err := capture.StreamJSONL(ctx, w, openCaptureStream, format); err != nil {
		return fmt.Errorf("running capture: %w", err)
	}

	return nil
}
