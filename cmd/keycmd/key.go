package keycmd

import (
	"fmt"

	this "github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/spf13/cobra"
)

var app **this.Avalanche

func SetupKeyCmd(injectedApp **this.Avalanche) *cobra.Command {
	app = injectedApp

	// subnet create
	keyCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&filename, "file", "", "file path of genesis to use instead of the wizard")
	createCmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")

	return keyCmd
}

// avalanche subnet
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Create and manage testnet signing keys",
	Long: `The key command suite provides a collection of tools for creating signing
keys. You can use these keys to deploy subnets to the Fuji testnet.

To get started, use the key create command.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
		}
	},
}
