/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List created subnet genesis files",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: listGenesis,
}

func init() {
	subnetCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const genesis_suffix = "_genesis.json"

func listGenesis(cmd *cobra.Command, args []string) {
	fmt.Println("Created subnet genesis files:")
	usr, _ := user.Current()
	mainDir := filepath.Join(usr.HomeDir, ".avalanche-cli")
	files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if strings.Contains(f.Name(), genesis_suffix) {
			prefixLen := len(f.Name()) - len(genesis_suffix)
			fmt.Println(f.Name()[:prefixLen])
		}
	}
}
