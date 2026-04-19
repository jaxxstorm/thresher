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
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var analyzeArgs struct {
	endpoint       string
	model          string
	input          string
	mode           string
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

func init() {
	rootCmd.AddCommand(newAnalyzeCommand())
	setAnalyzeDefaults()
}

func setAnalyzeDefaults() {
	viper.SetDefault("analyze.endpoint", "http://ai")
	viper.SetDefault("analyze.mode", "console")
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
			return runAnalyze(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&analyzeArgs.endpoint, "endpoint", "", "Aperture base endpoint, for example http://ai")
	cmd.Flags().StringVar(&analyzeArgs.model, "model", "", "model identifier to use for analysis")
	cmd.Flags().StringVarP(&analyzeArgs.input, "input", "i", "", "path to a saved JSONL packet stream to analyze instead of live capture")
	cmd.Flags().StringVar(&analyzeArgs.mode, "mode", "", "analysis presentation mode: console or web")
	cmd.Flags().StringVar(&analyzeArgs.webAccess, "web-access", "", "web exposure mode when --mode web: local or tailnet")
	cmd.Flags().StringVar(&analyzeArgs.endpointStyle, "endpoint-style", "", "Aperture endpoint shape: auto, messages, chat-completions, responses")
	cmd.Flags().IntVar(&analyzeArgs.batchPackets, "batch-packets", 0, "maximum packets per analysis batch")
	cmd.Flags().IntVar(&analyzeArgs.batchBytes, "batch-bytes", 0, "maximum encoded bytes per analysis batch")
	cmd.Flags().IntVar(&analyzeArgs.sessionPackets, "session-packets", 0, "maximum packets sent during one analysis session")
	cmd.Flags().IntVar(&analyzeArgs.sessionBytes, "session-bytes", 0, "maximum encoded bytes sent during one analysis session")
	cmd.Flags().IntVar(&analyzeArgs.maxTokens, "max-tokens", 0, "maximum tokens requested per model response")
	return cmd
}

func runAnalyze(ctx context.Context, stdout, stderr io.Writer) error {
	config := analyze.Config{
		Endpoint:       firstNonEmpty(analyzeArgs.endpoint, viper.GetString("analyze.endpoint")),
		Model:          firstNonEmpty(analyzeArgs.model, viper.GetString("analyze.model")),
		UserAgent:      resolveAnalyzeUserAgent(),
		WebAccess:      analyze.WebAccess(firstNonEmpty(analyzeArgs.webAccess, viper.GetString("analyze.web_access"))),
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

	mode := firstNonEmpty(analyzeArgs.mode, viper.GetString("analyze.mode"))
	if mode == "" {
		mode = "console"
	}
	if mode != "console" && mode != "web" {
		return fmt.Errorf("invalid analysis mode %q: expected console or web", mode)
	}
	if config.WebAccess == "" {
		config.WebAccess = analyze.WebAccessLocal
	}
	if config.WebAccess != analyze.WebAccessLocal && config.WebAccess != analyze.WebAccessTailnet {
		return fmt.Errorf("invalid analysis web access %q: expected local or tailnet", config.WebAccess)
	}

	session := analyze.NewSession(config)
	presenter := analyze.Presenter(analyze.NewConsolePresenter(config))
	if mode == "web" {
		webPresenter := newAnalyzeWebPresenter(config)
		presenter = webPresenter
		go func() {
			url := <-webPresenter.Ready()
			_, _ = fmt.Fprintf(stderr, "analyze started; endpoint=%s model=%s mode=%s web-access=%s url=%s\n", config.Endpoint, config.Model, mode, config.WebAccess, url)
		}()
	} else if !isInteractiveAnalyzeSession() {
		if _, err := fmt.Fprintf(stderr, "analyze started; endpoint=%s model=%s mode=%s\n", config.Endpoint, config.Model, mode); err != nil {
			return fmt.Errorf("writing analyze status: %w", err)
		}
	}
	_ = stdout
	if analyzeArgs.input != "" {
		file, err := os.Open(analyzeArgs.input)
		if err != nil {
			return fmt.Errorf("opening analyze input: %w", err)
		}
		defer file.Close()
		return session.RunReaderWithPresenter(ctx, file, presenter)
	}
	return session.RunLiveWithPresenter(ctx, openAnalyzeCaptureStream, presenter)
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
