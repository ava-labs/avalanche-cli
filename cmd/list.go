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
	"sort"
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
}

const genesis_suffix = "_genesis.json"

type subnetMatrix [][]string

func (c subnetMatrix) Len() int      { return len(c) }
func (c subnetMatrix) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Compare strings by first key of the sub-slice
func (c subnetMatrix) Less(i, j int) bool { return strings.Compare(c[i][0], c[j][0]) == -1 }

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

	rows := subnetMatrix{}

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

			rows = append(rows, []string{sc.Subnet, sc.Name, string(sc.Vm)})
		}
	}
	sort.Sort(rows)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}
