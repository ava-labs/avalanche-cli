package keycmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/spf13/cobra"
)

const (
	forceFlag = "force"
)

var (
	forceCreate bool
	filename    string
)

var createCmd = &cobra.Command{
	Use:   "create [keyName]",
	Short: "Create a signing key",
	Long:  `Generates a new private key to use for creating and controlling test subnets.`,
	Args:  cobra.ExactArgs(1),
	RunE:  createKey,
}

func createKey(cmd *cobra.Command, args []string) error {
	keyName := args[0]

	if (*app).KeyExists(keyName) && !forceCreate {
		return errors.New("key already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	keyPath := (*app).GetKeyPath(keyName)

	k, err := key.NewSoft(0)
	if err != nil {
		return err
	}
	if err := k.Save(keyPath); err != nil {
		return err
	}
	return nil
}
