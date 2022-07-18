package keycmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [keyName]",
		Short: "Exports a signing key",
		Long: `Exports a created signing key. By default, the tool writes the hex encoded key to
stdout. If the --output flag is provided, the key will be written to a file of
your choosing.`,
		Args:         cobra.ExactArgs(1),
		RunE:         exportKey,
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(
		&filename,
		"output",
		"o",
		"",
		"write the key to the provided file path",
	)

	return cmd
}

func exportKey(cmd *cobra.Command, args []string) error {
	keyName := args[0]

	keyPath := app.GetKeyPath(keyName)
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}

	if filename == "" {
		fmt.Println(string(keyBytes))
		return nil
	}

	return os.WriteFile(filename, keyBytes, application.WriteReadReadPerms)
}
