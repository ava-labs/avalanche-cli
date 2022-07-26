// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/cmd/backendcmd"
	"github.com/ava-labs/avalanche-cli/cmd/keycmd"
	"github.com/ava-labs/avalanche-cli/cmd/networkcmd"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	app *application.Avalanche

	logLevel string
	Version  = ""
	cfgFile  string
)

func NewRootCmd() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use: "avalanche",
		Long: `Avalanche CLI is a command line tool that gives developers access to
everything Avalanche. This beta release specializes in helping developers
build and test subnets.

To get started, look at the documentation for the subcommands or jump right
in with avalanche subnet create myNewSubnet.`,
		PersistentPreRunE: createApp,
		Version:           Version,
	}

	// Disable printing the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.avalanche-cli.json)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "ERROR", "log level for the application")

	// add sub commands
	rootCmd.AddCommand(subnetcmd.NewCmd(app))
	rootCmd.AddCommand(networkcmd.NewCmd(app))
	rootCmd.AddCommand(keycmd.NewCmd(app))

	// add hidden backend command
	rootCmd.AddCommand(backendcmd.NewCmd(app))

	return rootCmd
}

func createApp(cmd *cobra.Command, args []string) error {
	baseDir, err := setupEnv()
	if err != nil {
		return err
	}
	log, err := setupLogging(baseDir)
	if err != nil {
		return err
	}
	cf := config.New()
	app.Setup(baseDir, log, cf, prompts.NewPrompter())
	cobra.OnInitialize(initConfig)
	return nil
}

func setupEnv() (string, error) {
	// Set base dir
	usr, err := user.Current()
	if err != nil {
		// no logger here yet
		fmt.Printf("unable to get system user %s\n", err)
		return "", err
	}
	baseDir := filepath.Join(usr.HomeDir, constants.BaseDirName)

	// Create base dir if it doesn't exist
	err = os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		// no logger here yet
		fmt.Printf("failed creating the basedir %s: %s\n", baseDir, err)
		return "", err
	}

	// Create snapshots dir if it doesn't exist
	snapshotsDir := filepath.Join(baseDir, constants.SnapshotsDirName)
	if err = os.MkdirAll(snapshotsDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the snapshots dir %s: %s\n", snapshotsDir, err)
		os.Exit(1)
	}

	// Create key dir if it doesn't exist
	keyDir := filepath.Join(baseDir, constants.KeyDir)
	if err = os.MkdirAll(keyDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the key dir %s: %s\n", keyDir, err)
		os.Exit(1)
	}

	return baseDir, nil
}

func setupLogging(baseDir string) (logging.Logger, error) {
	var err error

	config := logging.Config{}
	config.LogLevel = logging.Info
	config.DisplayLevel, err = logging.ToLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level configured: %s", logLevel)
	}
	config.Directory = filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(config.Directory, perms.ReadWriteExecute); err != nil {
		return nil, fmt.Errorf("failed creating log directory: %w", err)
	}

	// some logging config params
	config.LogFormat = logging.Colors
	config.MaxSize = constants.MaxLogFileSize
	config.MaxFiles = constants.MaxNumOfLogFiles
	config.MaxAge = constants.RetainOldFiles

	factory := logging.NewFactory(config)
	log, err := factory.Make("avalanche")
	if err != nil {
		factory.Close()
		return nil, fmt.Errorf("failed setting up logging, exiting: %s", err)
	}
	// create the user facing logger as a global var
	ux.NewUserLog(log, os.Stdout)
	return log, nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for default config.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType(constants.DefaultConfigFileType)
		viper.SetConfigName(constants.DefaultConfigFileName)
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		app.Log.Info("Using config file: %s", viper.ConfigFileUsed())
	} else {
		app.Log.Info("No log file found")
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	app = application.New()
	rootCmd := NewRootCmd()
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
