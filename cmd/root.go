// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/primarycmd"

	"github.com/ava-labs/avalanche-cli/cmd/nodecmd"

	"github.com/ava-labs/avalanche-cli/cmd/configcmd"

	"github.com/ava-labs/avalanche-cli/cmd/backendcmd"
	"github.com/ava-labs/avalanche-cli/cmd/keycmd"
	"github.com/ava-labs/avalanche-cli/cmd/networkcmd"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/cmd/transactioncmd"
	"github.com/ava-labs/avalanche-cli/cmd/updatecmd"
	"github.com/ava-labs/avalanche-cli/internal/migrations"
	"github.com/ava-labs/avalanche-cli/pkg/apmintegration"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	app *application.Avalanche

	logLevel  string
	Version   = ""
	cfgFile   string
	skipCheck bool
)

func NewRootCmd() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use: "avalanche",
		Long: `Avalanche-CLI is a command-line tool that gives developers access to
everything Avalanche. This release specializes in helping developers
build and test Subnets.

To get started, look at the documentation for the subcommands or jump right
in with avalanche subnet create myNewSubnet.`,
		PersistentPreRunE: createApp,
		Version:           Version,
		PersistentPostRun: handleTracking,
	}

	// Disable printing the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.avalanche-cli/config)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "ERROR", "log level for the application")
	rootCmd.PersistentFlags().BoolVar(&skipCheck, constants.SkipUpdateFlag, false, "skip check for new versions")

	// add sub commands
	rootCmd.AddCommand(subnetcmd.NewCmd(app))
	rootCmd.AddCommand(primarycmd.NewCmd(app))
	rootCmd.AddCommand(networkcmd.NewCmd(app))
	rootCmd.AddCommand(keycmd.NewCmd(app))

	// add hidden backend command
	rootCmd.AddCommand(backendcmd.NewCmd(app))

	// add transaction command
	rootCmd.AddCommand(transactioncmd.NewCmd(app))

	// add config command
	rootCmd.AddCommand(configcmd.NewCmd(app))

	// add update command
	rootCmd.AddCommand(updatecmd.NewCmd(app, Version))

	// add node command
	rootCmd.AddCommand(nodecmd.NewCmd(app))
	return rootCmd
}

func createApp(cmd *cobra.Command, _ []string) error {
	baseDir, err := setupEnv()
	if err != nil {
		return err
	}
	log, err := setupLogging(baseDir)
	if err != nil {
		return err
	}
	cf := config.New()
	app.Setup(baseDir, log, cf, prompts.NewPrompter(), application.NewDownloader())

	// Setup APM, skip if running a hidden command
	if !cmd.Hidden {
		usr, err := user.Current()
		if err != nil {
			app.Log.Error("unable to get system user")
			return err
		}
		apmBaseDir := filepath.Join(usr.HomeDir, constants.APMDir)
		if err = apmintegration.SetupApm(app, apmBaseDir); err != nil {
			return err
		}
	}

	initConfig()

	if err := migrations.RunMigrations(app); err != nil {
		return err
	}

	if os.Getenv("RUN_E2E") == "" && !app.ConfigFileExists("") {
		err = utils.HandleUserMetricsPreference(app)
		if err != nil {
			return err
		}
	}
	if err := checkForUpdates(cmd, app); err != nil {
		return err
	}

	return nil
}

// checkForUpdates evaluates first if the user is maybe wanting to skip the update check
// if there's no skip, it runs the update check
func checkForUpdates(cmd *cobra.Command, app *application.Avalanche) error {
	var (
		lastActs *application.LastActions
		err      error
	)
	// we store a timestamp of the last skip check in a file
	lastActs, err = app.ReadLastActionsFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the file does not exist AND the user is requesting to skipCheck,
			// we write the new file
			if skipCheck {
				lastActs := &application.LastActions{
					LastSkipCheck: time.Now(),
				}
				app.WriteLastActionsFile(lastActs)
				return nil
			}
		}
		app.Log.Warn("failed to read last-actions file! This is non-critical but is logged", zap.Error(err))
		lastActs = &application.LastActions{}
	}

	// if the user had requested to skipCheck less than 24 hrs ago, we skip in any case
	if lastActs.LastSkipCheck != (time.Time{}) &&
		time.Now().Before(lastActs.LastSkipCheck.Add(24*time.Hour)) {
		app.Log.Debug("last checked %s, so less than 24 hrs earlier. Skipping to check for updates.",
			zap.Time("lastSkipCheck", lastActs.LastSkipCheck))
		return nil
	}

	// more than 24hrs ago or the user never asked to skip before
	// we update the timestamp and write the file again
	if skipCheck {
		if lastActs == nil {
			lastActs = &application.LastActions{}
		}
		lastActs.LastSkipCheck = time.Now()
		app.WriteLastActionsFile(lastActs)
		return nil
	}

	// at this point we want to run the check
	isUserCalled := false
	commandList := strings.Fields(cmd.CommandPath())
	if !(len(commandList) > 1 && commandList[1] == "update") {
		if lastActs.LastCheckGit != (time.Time{}) && time.Now().Before(lastActs.LastCheckGit.Add(24*time.Hour)) {
			if err := updatecmd.Update(cmd, isUserCalled, Version, lastActs); err != nil {
				if errors.Is(err, updatecmd.ErrUserAbortedInstallation) {
					return nil
				}
				if err == updatecmd.ErrNoVersion {
					ux.Logger.PrintToUser(
						"Attempted to check if a new version is available, but couldn't find the currently running version information")
					ux.Logger.PrintToUser(
						"Make sure to follow official instructions, or automatic updates won't be available for you")
					return nil
				}
				return err
			}
		}
	}
	return nil
}

func handleTracking(cmd *cobra.Command, _ []string) {
	utils.HandleTracking(cmd, app, nil)
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

	// Create custom vm dir if it doesn't exist
	vmDir := filepath.Join(baseDir, constants.CustomVMDir)
	if err = os.MkdirAll(vmDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the vm dir %s: %s\n", vmDir, err)
		os.Exit(1)
	}

	// Create subnet dir if it doesn't exist
	subnetDir := filepath.Join(baseDir, constants.SubnetDir)
	if err = os.MkdirAll(subnetDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the subnet dir %s: %s\n", subnetDir, err)
		os.Exit(1)
	}

	// Create repos dir if it doesn't exist
	repoDir := filepath.Join(baseDir, constants.ReposDir)
	if err = os.MkdirAll(repoDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the repo dir %s: %s\n", repoDir, err)
		os.Exit(1)
	}

	pluginDir := filepath.Join(baseDir, constants.PluginDir)
	if err = os.MkdirAll(pluginDir, os.ModePerm); err != nil {
		fmt.Printf("failed creating the plugin dir %s: %s\n", pluginDir, err)
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
	config.Directory = filepath.Join(baseDir, constants.LogDir)
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
		return nil, fmt.Errorf("failed setting up logging, exiting: %w", err)
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
		viper.AddConfigPath(fmt.Sprintf("%s/%s", home, constants.BaseDirName))
		viper.SetConfigName(constants.DefaultConfigFileName)
		viper.SetConfigType(constants.DefaultConfigFileType)
		//migrate old config
		oldConfig := fmt.Sprintf("%s/%s.%s", home, constants.OldConfigFileName, constants.DefaultConfigFileType)
		if app.ConfigFileExists(oldConfig) {
			ux.Logger.PrintToUser("-----------------------------------------------------------------------")
			ux.Logger.PrintToUser("WARNING: Depricated configuration file was found in %s", oldConfig)
			ux.Logger.PrintToUser("Please run avalanche config migrate to migrate it to new default location %s", constants.DefaultConfigFileName)
			ux.Logger.PrintToUser("-----------------------------------------------------------------------")
		} else {
			return
		}

	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		app.Log.Info("Using config file", zap.String("config-file", viper.ConfigFileUsed()))
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
