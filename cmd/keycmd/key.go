package keycmd

import (
	"fmt"

	this "github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/spf13/cobra"
)

var app **this.Avalanche

func NewCmd(injectedApp **this.Avalanche) *cobra.Command {
	app = injectedApp

	cmd := &cobra.Command{
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

	// subnet create
	cmd.AddCommand(newCreateCmd())

	return cmd
}
