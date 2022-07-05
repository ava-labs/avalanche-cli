package keycmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

const (
	forceFlag = "force"
)

var (
	forceCreate bool
	filename    string
)

func createKey(cmd *cobra.Command, args []string) error {
	keyName := args[0]

	if (*app).KeyExists(keyName) && !forceCreate {
		return errors.New("key already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if filename == "" {
		// Create key from scratch
		ux.Logger.PrintToUser("Generating new key...")
		k, err := key.NewSoft(0)
		if err != nil {
			return err
		}
		keyPath := (*app).GetKeyPath(keyName)
		if err := k.Save(keyPath); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Key created")
	} else {
		// Load key from file
		// TODO add validation that key is legal
		ux.Logger.PrintToUser("Loading user key...")
		if err := (*app).CopyKeyFile(filename, keyName); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Key loaded")
	}

	return nil
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create [keyName]",
		Short:        "Create a signing key",
		Long:         `Generates a new private key to use for creating and controlling test subnets.`,
		Args:         cobra.ExactArgs(1),
		RunE:         createKey,
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&filename, "file", "", "file path of genesis to use instead of the wizard")
	cmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")
	return cmd
}
