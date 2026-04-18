package cmd

import (
	"fmt"

	"github.com/jaxxstorm/vers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/go-git/go-git/v5/plumbing"
)

var ReleaseVersion string

func init() {
	rootCmd.AddCommand(newVersionCommand())
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print the local repository version",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			versions, err := calculateVersion()
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), versions.Go)
			return err
		},
	}
}

func calculateVersion() (*vers.LanguageVersions, error) {
	if ReleaseVersion != "" {
		return &vers.LanguageVersions{Go: ReleaseVersion}, nil
	}

	repoPath := viper.GetString("repo-path")
	if repoPath == "" {
		repoPath = "."
	}

	repo, err := vers.OpenRepository(repoPath)
	if err != nil {
		return vers.GenerateFallbackVersion(), nil
	}

	versions, err := vers.Calculate(vers.Options{
		Repository: repo,
		Commitish:  plumbing.Revision("HEAD"),
	})
	if err != nil {
		return vers.GenerateFallbackVersion(), nil
	}

	return versions, nil
}
