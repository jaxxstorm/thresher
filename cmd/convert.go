package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaxxstorm/thresher/internal/capture"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newConvertCommand())
}

func newConvertCommand() *cobra.Command {
	var output string
	var mode string
	var includeRaw bool
	cmd := &cobra.Command{
		Use:           "convert <capture.pcap>",
		Short:         "Convert a Tailscale debug PCAP to JSON for LLM upload",
		Long:          "Convert a Tailscale debug-capture PCAP (USER0) into a single JSON document.\nRuns offline; no data is uploaded. Raw hex and payload previews are omitted by\ndefault, but addresses, DNS names, and other sensitive metadata remain.\nGeneral Ethernet PCAPs and PCAPNG are not supported.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			exportMode := mode
			if summary {
				if cmd.Flags().Changed("mode") && mode != "summary" {
					return fmt.Errorf("--summary cannot be combined with --mode %s", mode)
				}
				if includeRaw {
					return fmt.Errorf("--include-raw=true is not supported with --summary")
				}
				exportMode = "summary"
			}
			path := output
			if path == "" {
				path = strings.TrimSuffix(args[0], filepath.Ext(args[0])) + ".json"
			}
			return runConvert(cmd.Context(), args[0], path, cmd.OutOrStdout(), exportMode, includeRaw)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file (default input name with .json extension; - for stdout)")
	cmd.Flags().StringVar(&mode, "mode", "detailed", "export mode: detailed or summary")
	_ = cmd.Flags().MarkHidden("mode")
	cmd.Flags().BoolVar(&includeRaw, "include-raw", false, "include raw packet hex and payload previews (larger and potentially sensitive)")
	return cmd
}

func runConvert(ctx context.Context, inputPath, outputPath string, stdout io.Writer, mode string, includeRaw bool) error {
	if mode != "detailed" && mode != "summary" {
		return fmt.Errorf("invalid mode %q: must be detailed or summary", mode)
	}
	if mode == "summary" && includeRaw {
		return fmt.Errorf("--include-raw=true is not supported with --mode summary")
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening input PCAP: %w", err)
	}
	defer input.Close()
	open := func(context.Context) (io.ReadCloser, error) { return input, nil }
	export := func(output io.Writer) error {
		if mode == "summary" {
			return capture.ExportSummaryJSON(ctx, output, open)
		}
		return capture.ExportJSON(ctx, output, open, includeRaw)
	}
	if outputPath == "-" {
		return export(stdout)
	}
	inputInfo, err := input.Stat()
	if err != nil {
		return fmt.Errorf("checking input PCAP: %w", err)
	}
	outputInfo, err := os.Stat(outputPath)
	if err == nil && os.SameFile(inputInfo, outputInfo) {
		return fmt.Errorf("input and output must be different files")
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("checking output file: %w", err)
	}
	// Publish only a complete export, preserving any existing output on failure.
	output, err := os.CreateTemp(filepath.Dir(outputPath), ".thresher-convert-*.json")
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer os.Remove(output.Name())
	defer output.Close()
	if err := export(output); err != nil {
		return fmt.Errorf("converting PCAP: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("closing output file: %w", err)
	}
	if err := os.Rename(output.Name(), outputPath); err != nil {
		return fmt.Errorf("publishing output file: %w", err)
	}
	return nil
}
