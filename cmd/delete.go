/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes a generated subnet genesis",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run:  deleteGenesis,
	Args: cobra.ExactArgs(1),
}

func init() {
	subnetCmd.AddCommand(deleteCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func deleteGenesis(cmd *cobra.Command, args []string) {
	usr, _ := user.Current()
	// TODO sanitize this input
	genesis := filepath.Join(usr.HomeDir, BaseDir, args[0]+genesis_suffix)
	sidecar := filepath.Join(usr.HomeDir, BaseDir, args[0]+sidecar_suffix)

	if _, err := os.Stat(genesis); err == nil {
		// exists
		os.Remove(genesis)
		os.Remove(sidecar)
	} else if errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		fmt.Println("Specified genesis does not exist")
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		fmt.Println(err)
	}

	if _, err := os.Stat(sidecar); err == nil {
		// exists
		os.Remove(sidecar)
		fmt.Println("Deleted subnet")
	} else if errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		fmt.Println("Specified sidecar does not exist")
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		fmt.Println(err)
	}
}
