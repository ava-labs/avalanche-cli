/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var baseDir string
var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "avalanche",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
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
		fmt.Println("Error: Unable to get system user")
		os.Exit(1)
	}
	baseDir = filepath.Join(usr.HomeDir, BaseDirName)

	// Create base dir if it doesn't exist
	err = os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.avalanche-cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

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
