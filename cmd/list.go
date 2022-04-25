/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/olekukonko/tablewriter"
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
	header := []string{"subnet", "chain", "type"}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.SetRowLine(true)

	usr, _ := user.Current()
	mainDir := filepath.Join(usr.HomeDir, BaseDir)
	files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(mainDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				fmt.Println(err)
				return
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				fmt.Println(err)
				return
			}

			// prefixLen := len(f.Name()) - len(sidecar_suffix)
			// fmt.Println(f.Name()[:prefixLen])
			table.Append([]string{sc.Name, sc.Name, string(sc.Vm)})
		}
	}
	table.Render()
}
