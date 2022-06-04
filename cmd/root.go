// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/spf13/cobra"
)

var (
	baseDir  string
	logLevel string

	Version = ""

	log logging.Logger

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use: "avalanche",
		Long: `Avalanche CLI is a command line tool that gives developers access to
everything Avalanche. This beta release specializes in helping developers
build and test subnets.

To get started, look at the documentation for the subcommands or jump right
in with avalanche subnet create myNewSubnet.`,
		PersistentPreRunE: setupLogging,
		Version:           Version,
	}

	snapshotsDir string
)

func setupLogging(cmd *cobra.Command, args []string) error {
	var err error

	config := logging.Config{}
	config.LogLevel = logging.Info
	config.DisplayLevel, err = logging.ToLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level configured: %s", logLevel)
	}
	config.Directory = filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(config.Directory, perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed creating log directory: %w", err)
	}

	// some logging config params
	config.LogFormat = logging.Colors
	config.MaxSize = maxLogFileSize
	config.MaxFiles = maxNumOfLogFiles
	config.MaxAge = retainOldFiles

	factory := logging.NewFactory(config)
	log, err = factory.Make("avalanche")
	if err != nil {
		factory.Close()
		return fmt.Errorf("failed setting up logging, exiting: %s", err)
	}
	// create the user facing logger as a global var
	ux.NewUserLog(log, os.Stdout)
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Set base dir
	usr, err := user.Current()
	if err != nil {
		// no logger here yet
		fmt.Printf("unable to get system user %s\n", err)
		os.Exit(1)
	}
	baseDir = filepath.Join(usr.HomeDir, BaseDirName)

	// Create base dir if it doesn't exist
	err = os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		// no logger here yet
		fmt.Printf("failed creating the basedir %s: %s\n", baseDir, err)
		os.Exit(1)
	}

	// Set snapshots dir
	snapshotsDir = filepath.Join(baseDir, snapshotsDirName)

	// Create snapshots dir if it doesn't exist
	err = os.MkdirAll(snapshotsDir, os.ModePerm)
	if err != nil {
		// no logger here yet
		fmt.Printf("failed creating the snapshots dir %s: %s\n", snapshotsDir, err)
		os.Exit(1)
	}

	// Copy default snapshot if not present
	defaultSnapshotPath := filepath.Join(snapshotsDir, "anr-snapshot-"+constants.DefaultSnapshotName)
	if _, err := os.Stat(defaultSnapshotPath); os.IsNotExist(err) {
		defaultSnapshotBytes, err := ioutil.ReadFile("defaultSnapshot.tar.gz")
		if err != nil {
			fmt.Printf("failed reading initial default snapshot: %w\n", err)
			os.Exit(1)
		}
		if err := binutils.InstallArchive("tar.gz", defaultSnapshotBytes, snapshotsDir); err != nil {
			fmt.Printf("failed installing initial default snapshot: %w\n", err)
			os.Exit(1)
		}
	}

	// Disable printing the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "ERROR", "log level for the application")

	// add sub commands
	rootCmd.AddCommand(subnetCmd)
	rootCmd.AddCommand(networkCmd)

	// add hidden backend command
	backendCmd.Hidden = true
	rootCmd.AddCommand(backendCmd)

	// subnet create
	subnetCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&filename, "file", "", "file path of genesis to use instead of the wizard")
	createCmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the SubnetEVM as the base template")
	createCmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	createCmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")

	// subnet delete
	subnetCmd.AddCommand(deleteCmd)

	// subnet deploy
	subnetCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")

	// subnet describe
	subnetCmd.AddCommand(readCmd)
	readCmd.Flags().BoolVarP(
		&printGenesisOnly,
		"genesis",
		"g",
		false,
		"Print the genesis to the console directly instead of the summary",
	)

	// subnet list
	subnetCmd.AddCommand(listCmd)

	// network
	// network start
	networkCmd.AddCommand(startCmd)

	// network stop
	networkCmd.AddCommand(stopCmd)

	// network clean
	networkCmd.AddCommand(cleanCmd)

	// network status
	networkCmd.AddCommand(statusCmd)
}
