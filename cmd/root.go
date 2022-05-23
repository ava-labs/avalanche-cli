// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/spf13/cobra"
)

var (
	baseDir  string
	cfgFile  string
	logLevel string

	Version = ""

	log logging.Logger

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:               "avalanche",
		Short:             "A brief description of your application",
		PersistentPreRunE: setupLogging,
		Version:           Version,
	}
)

func setupLogging(cmd *cobra.Command, args []string) error {
	var err error

	config := logging.DefaultConfig
	config.DisplayLevel, err = logging.ToLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level configured: %s", logLevel)
	}
	config.Directory = filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(config.Directory, perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed creating log directory: %w", err)
	}
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

	// Disable printing the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "ERROR", "log level for the application")

	// add sub commands
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(subnetCmd)

	// add hidden backend command
	backendCmd.Hidden = true
	rootCmd.AddCommand(backendCmd)

	// subnet create
	subnetCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&filename, "file", "", "filepath of genesis to use")
	createCmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the SubnetEVM as your VM")
	createCmd.Flags().BoolVar(&useCustom, "custom", false, "use your own custom VM as your VM")
	createCmd.Flags().BoolVarP(&forceCreate, "force", "f", false, "overwrite the existing genesis if one exists")

	// subnet delete
	subnetCmd.AddCommand(deleteCmd)

	// subnet deploy
	subnetCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "Deploy subnet locally")
	deployCmd.Flags().BoolVarP(&force, "force", "f", false, "Deploy without asking for confirmation")

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
}
