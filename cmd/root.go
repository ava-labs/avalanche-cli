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

	"github.com/ava-labs/avalanche-cli/cmd/backendcmd"
	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/cmd/configcmd"
	"github.com/ava-labs/avalanche-cli/cmd/contractcmd"
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd"
	"github.com/ava-labs/avalanche-cli/cmd/interchaincmd/tokentransferrercmd"
	"github.com/ava-labs/avalanche-cli/cmd/keycmd"
	"github.com/ava-labs/avalanche-cli/cmd/networkcmd"
	"github.com/ava-labs/avalanche-cli/cmd/nodecmd"
	"github.com/ava-labs/avalanche-cli/cmd/primarycmd"
	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd"
	"github.com/ava-labs/avalanche-cli/cmd/transactioncmd"
	"github.com/ava-labs/avalanche-cli/cmd/updatecmd"
	"github.com/ava-labs/avalanche-cli/internal/migrations"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/metrics"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	app       *application.Avalanche
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
		SilenceErrors:     true,
		SilenceUsage:      true,
	}

	// Disable printing the completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().
		StringVar(&cfgFile, "config", "", "config file (default is $HOME/.avalanche-cli/config.json)")
	rootCmd.PersistentFlags().
		StringVar(&logLevel, "log-level", "ERROR", "log level for the application")
	rootCmd.PersistentFlags().
		BoolVar(&skipCheck, constants.SkipUpdateFlag, false, "skip check for new versions")

	// add sub commands
	rootCmd.AddCommand(blockchaincmd.NewCmd(app))
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

	// add teleporter command
	rootCmd.AddCommand(teleportercmd.NewCmd(app))

	// add interchain command
	rootCmd.AddCommand(interchaincmd.NewCmd(app))

	// add ictt command
	subcmd := tokentransferrercmd.NewCmd(app)
	subcmd.Use = "ictt"
	subcmd.Short = "Manage Interchain Token Transferrers (shorthand for `interchain TokenTransferrer`)"
	subcmd.Long = "The ictt command suite provides tools to deploy and manage Interchain Token Transferrers."
	rootCmd.AddCommand(subcmd)

	// add subnet command
	subcmd = blockchaincmd.NewCmd(app)
	subcmd.Use = "subnet"
	subcmd.Short = "Create and deploy blockchains (deprecation notice: use 'avalanche blockchain')"
	subcmd.Long = `The subnet command suite provides a collection of tools for developing
and deploying Blockchains.

To get started, use the subnet create command wizard to walk through the
configuration of your very first Blockchain. Then, go ahead and deploy it
with the subnet deploy command. You can use the rest of the commands to
manage your Blockchain configurations and live deployments.

Deprecation notice: use 'avalanche blockchain'`
	rootCmd.AddCommand(subcmd)

	// add contract command
	rootCmd.AddCommand(contractcmd.NewCmd(app))

	cobrautils.ConfigureRootCmd(rootCmd)

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
	log.Info("-----------")
	log.Info(fmt.Sprintf("cmd: %s", strings.Join(os.Args[1:], " ")))
	cf := config.New()
	app.Setup(baseDir, log, cf, prompts.NewPrompter(), application.NewDownloader())

	initConfig()

	if err := migrations.RunMigrations(app); err != nil {
		return err
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
		app.Log.Warn(
			"failed to read last-actions file! This is non-critical but is logged",
			zap.Error(err),
		)
		lastActs = &application.LastActions{}
	}

	// if the user had requested to skipCheck less than 24 hrs ago, we skip in any case
	if lastActs.LastSkipCheck != (time.Time{}) &&
		time.Now().Before(lastActs.LastSkipCheck.Add(24*time.Hour)) {
		app.Log.Debug(
			"last checked %s, so less than 24 hrs earlier. Skipping to check for updates.",
			zap.Time("lastSkipCheck", lastActs.LastSkipCheck),
		)
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
		if lastActs.LastCheckGit == (time.Time{}) || time.Now().After(lastActs.LastCheckGit.Add(24*time.Hour)) {
			if err := updatecmd.Update(cmd, isUserCalled, Version, lastActs); err != nil {
				if errors.Is(err, updatecmd.ErrUserAbortedInstallation) {
					return nil
				}
				if err == updatecmd.ErrNoVersion {
					ux.Logger.PrintToUser(
						"Attempted to check if a new version is available, but couldn't find the currently running version information",
					)
					ux.Logger.PrintToUser(
						"Make sure to follow official instructions, or automatic updates won't be available for you",
					)
					return nil
				}
				return err
			}
		}
	}
	return nil
}

func handleTracking(cmd *cobra.Command, _ []string) {
	metrics.HandleTracking(cmd, cmd.CommandPath(), app, nil)
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
	oldMetricsConfig := utils.UserHomePath(constants.OldMetricsConfigFileName)
	if cfgFile == "" {
		cfgFile = utils.UserHomePath(constants.DefaultConfigFileName)
	}
	app.Conf.SetConfig(app.Log, cfgFile)
	// check if metrics setting is available, and if not load metricConfig
	if !app.Conf.ConfigValueIsSet(constants.ConfigMetricsEnabledKey) {
		if utils.FileExists(oldMetricsConfig) {
			app.Conf.MergeConfig(app.Log, oldMetricsConfig)
		}
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	app = application.New()
	rootCmd := NewRootCmd()
	err := rootCmd.Execute()
	cobrautils.HandleErrors(err)
}
