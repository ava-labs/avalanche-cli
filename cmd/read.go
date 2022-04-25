/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read created subnet genesis file",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run:  readGenesis,
	Args: cobra.ExactArgs(1),
}

func init() {
	subnetCmd.AddCommand(readCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func readGenesis(cmd *cobra.Command, args []string) {
	usr, _ := user.Current()
	genesisFile := filepath.Join(usr.HomeDir, ".avalanche-cli", args[0]+genesis_suffix)
	gen, err := os.ReadFile(genesisFile)
	// files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(gen))
}
