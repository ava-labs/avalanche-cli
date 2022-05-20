// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	baseDir  string
	cfgFile  string
	logLevel string

	Version = ""
	log     logging.Logger

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
	log, err = factory.Make("main")
	if err != nil {
		factory.Close()
		return fmt.Errorf("failed setting up logging, exiting: %s", err)
	}
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
	cobra.OnInitialize(initConfig)

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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.avalanche-cli.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "ERROR", "log level for the application")

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// add sub commands
	rootCmd.AddCommand(backendCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(subnetCmd)

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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".avalanche-cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".avalanche-cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
