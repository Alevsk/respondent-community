package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Alevsk/respondent/embedfs"
	"github.com/Alevsk/respondent/internal/ui"
)

func init() {
	// Populate the embedded frontend filesystem from the embedfs package.
	// The embedfs package lives at the module root, where the //go:embed directive
	// can legally reference frontend/apps/earth/dist.
	ui.EarthDist = embedfs.EarthDistFS()
}

var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "respondent-community",
		Short: "Respondent Community Edition — single-binary deployment",
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Route config through the shared InitViper (env prefix, AutomaticEnv,
		// shared + community defaults, config-file read) — mirrors the canonical
		// per-service pattern. See internal/app/respondent/config/viper.go.
		if err := InitViper(cfgFile); err != nil {
			// ConfigFileNotFoundError is only returned by viper's implicit search
			// (no --config flag). When that search finds nothing, tolerate it and
			// run on defaults + env. An explicit --config <missing> surfaces as a
			// generic *os.PathError (not ConfigFileNotFoundError), so it falls
			// through to the fatal return below.
			var notFound viper.ConfigFileNotFoundError
			if errors.As(err, &notFound) {
				return nil
			}
			return fmt.Errorf("init config: %w", err)
		}
		return nil
	}

	rootCmd.AddCommand(newServeCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
