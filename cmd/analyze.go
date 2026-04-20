package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jaxxstorm/thresher/internal/analyze"
	"github.com/jaxxstorm/thresher/internal/capture"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var analyzeArgs struct {
	endpoint       string
	model          string
	input          string
	webAccess      string
	endpointStyle  string
	batchPackets   int
	batchBytes     int
	sessionPackets int
	sessionBytes   int
	maxTokens      int
}

var openAnalyzeCaptureStream = capture.OpenLocalAPIStream

type analyzeWebPresenter interface {
	analyze.Presenter
	Ready() <-chan string
}

var newAnalyzeWebPresenter = func(config analyze.Config) analyzeWebPresenter {
	return analyze.NewWebPresenter(config)
}

const (
	analyzeModeConsole = "console"
	analyzeModeWeb     = "web"
)

func init() {
	rootCmd.AddCommand(newAnalyzeCommand())
	setAnalyzeDefaults()
}

func setAnalyzeDefaults() {
	viper.SetDefault("analyze.endpoint", "http://ai")
	viper.SetDefault("analyze.web_access", string(analyze.WebAccessLocal))
	viper.SetDefault("analyze.endpoint_style", string(analyze.EndpointAuto))
	viper.SetDefault("analyze.batch_packets", 20)
	viper.SetDefault("analyze.batch_bytes", 64*1024)
	viper.SetDefault("analyze.session_packets", 500)
	viper.SetDefault("analyze.session_bytes", 2*1024*1024)
	viper.SetDefault("analyze.max_tokens", 300)
}

func newAnalyzeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "analyze",
		Aliases:       []string{"analyse"},
		Short:         "Analyze decoded capture traffic with an Aperture-served LLM",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnalyzeWithMode(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), analyzeModeConsole)
		},
	}

	addSharedAnalyzeFlags(cmd.PersistentFlags())
	cmd.AddCommand(newAnalyzeConsoleCommand(), newAnalyzeWebCommand())
	return cmd
}

func newAnalyzeConsoleCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "console",
		Aliases:       []string{"local"},
		Short:         "Run analyze in the fullscreen console workflow",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnalyzeWithMode(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), analyzeModeConsole)
		},
	}
}

func newAnalyzeWebCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "web",
		Short:         "Run analyze in the browser workflow",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnalyzeWithMode(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), analyzeModeWeb)
		},
	}

	cmd.Flags().StringVar(&analyzeArgs.webAccess, "web-access", "", "web exposure mode: local or tailnet")
	return cmd
}

func addSharedAnalyzeFlags(flags *pflag.FlagSet) {
	flags.StringVar(&analyzeArgs.endpoint, "endpoint", "", "Aperture base endpoint, for example http://ai")
	flags.StringVar(&analyzeArgs.model, "model", "", "model identifier to use for analysis")
	flags.StringVarP(&analyzeArgs.input, "input", "i", "", "path to a saved JSONL packet stream to analyze instead of live capture")
	flags.StringVar(&analyzeArgs.endpointStyle, "endpoint-style", "", "Aperture endpoint shape: auto, messages, chat-completions, responses")
	flags.IntVar(&analyzeArgs.batchPackets, "batch-packets", 0, "maximum packets per analysis batch")
	flags.IntVar(&analyzeArgs.batchBytes, "batch-bytes", 0, "maximum encoded bytes per analysis batch")
	flags.IntVar(&analyzeArgs.sessionPackets, "session-packets", 0, "maximum packets sent during one analysis session")
	flags.IntVar(&analyzeArgs.sessionBytes, "session-bytes", 0, "maximum encoded bytes sent during one analysis session")
	flags.IntVar(&analyzeArgs.maxTokens, "max-tokens", 0, "maximum tokens requested per model response")
}

func runAnalyze(ctx context.Context, stdout, stderr io.Writer) error {
	return runAnalyzeWithMode(ctx, stdout, stderr, analyzeModeConsole)
}

func runAnalyzeWithMode(ctx context.Context, stdout, stderr io.Writer, mode string) error {
	config := analyze.Config{
		Endpoint:       firstNonEmpty(analyzeArgs.endpoint, viper.GetString("analyze.endpoint")),
		Model:          firstNonEmpty(analyzeArgs.model, viper.GetString("analyze.model")),
		UserAgent:      resolveAnalyzeUserAgent(),
		EndpointStyle:  analyze.EndpointStyle(firstNonEmpty(analyzeArgs.endpointStyle, viper.GetString("analyze.endpoint_style"))),
		BatchPackets:   firstNonZero(analyzeArgs.batchPackets, viper.GetInt("analyze.batch_packets")),
		BatchBytes:     firstNonZero(analyzeArgs.batchBytes, viper.GetInt("analyze.batch_bytes")),
		SessionPackets: firstNonZero(analyzeArgs.sessionPackets, viper.GetInt("analyze.session_packets")),
		SessionBytes:   firstNonZero(analyzeArgs.sessionBytes, viper.GetInt("analyze.session_bytes")),
		MaxTokens:      firstNonZero(analyzeArgs.maxTokens, viper.GetInt("analyze.max_tokens")),
	}

	if config.Model == "" {
		return fmt.Errorf("analysis model required: pass --model or configure analyze.model")
	}
	if mode != analyzeModeConsole && mode != analyzeModeWeb {
		return fmt.Errorf("invalid analysis mode %q: expected console or web", mode)
	}

	if mode == analyzeModeWeb {
		config.WebAccess = analyze.WebAccess(firstNonEmpty(analyzeArgs.webAccess, viper.GetString("analyze.web_access")))
		if config.WebAccess == "" {
			config.WebAccess = analyze.WebAccessLocal
		}
		if config.WebAccess != analyze.WebAccessLocal && config.WebAccess != analyze.WebAccessTailnet {
			return fmt.Errorf("invalid analysis web access %q: expected local or tailnet", config.WebAccess)
		}
	}

	session := analyze.NewSession(config)
	presenter := analyze.Presenter(analyze.NewConsolePresenter(config))
	runSession := func(p analyze.Presenter) error {
		if analyzeArgs.input != "" {
			file, err := os.Open(analyzeArgs.input)
			if err != nil {
				return fmt.Errorf("opening analyze input: %w", err)
			}
			defer file.Close()
			return session.RunReaderWithPresenter(ctx, file, p)
		}
		return session.RunLiveWithPresenter(ctx, openAnalyzeCaptureStream, p)
	}

	if mode == analyzeModeWeb {
		webPresenter := newAnalyzeWebPresenter(config)
		presenter = webPresenter
		errCh := make(chan error, 1)
		go func() {
			errCh <- runSession(webPresenter)
		}()

		select {
		case url := <-webPresenter.Ready():
			if _, err := fmt.Fprintf(stderr, "analyze started; endpoint=%s model=%s mode=%s web-access=%s url=%s\n", config.Endpoint, config.Model, mode, config.WebAccess, url); err != nil {
				return fmt.Errorf("writing analyze status: %w", err)
			}
			return <-errCh
		case err := <-errCh:
			return err
		}
	} else if !isInteractiveAnalyzeSession() {
		if _, err := fmt.Fprintf(stderr, "analyze started; endpoint=%s model=%s mode=%s\n", config.Endpoint, config.Model, mode); err != nil {
			return fmt.Errorf("writing analyze status: %w", err)
		}
	}

	_ = stdout
	return runSession(presenter)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func isInteractiveAnalyzeSession() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd())) &&
		term.IsTerminal(int(os.Stderr.Fd()))
}

func resolveAnalyzeUserAgent() string {
	versions, err := calculateVersion()
	if err != nil || versions == nil {
		return formatAnalyzeUserAgent("")
	}
	return formatAnalyzeUserAgent(versions.Go)
}

func formatAnalyzeUserAgent(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}
	return "thresher/" + version
}
