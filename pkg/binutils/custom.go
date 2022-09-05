// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import "github.com/ava-labs/avalanche-cli/pkg/application"

func SetupCustomBin(app *application.Avalanche, subnetName string) string {
	// Just need to get the path of the vm
	return app.GetCustomVMPath(subnetName)
}

func SetupAPMBin(app *application.Avalanche, vmid string) string {
	// Just need to get the path of the vm
	return app.GetAPMVMPath(vmid)
}
