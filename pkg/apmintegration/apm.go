package apmintegration

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

// Returns alias
func AddRepo(app *application.Avalanche, repoURL string, branch string) (string, error) {
	alias, err := getAlias(repoURL)
	if err != nil {
		return "", err
	}

	if alias == constants.DefaultAvaLabsPackage {
		ux.Logger.PrintToUser("Avalanche Plugins Core already installed, skipping...")
		return "", nil
	}

	return alias, app.Apm.AddRepository(alias, repoURL, branch)
}

func InstallVM(app *application.Avalanche, subnet string) error {
	vms, err := getVMsInSubnet(app, subnet)
	if err != nil {
		return err
	}

	for _, vm := range vms {
		err = app.Apm.Install(vm)
		if err != nil {
			return err
		}
	}

	return nil
}
