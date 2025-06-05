// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package version

import (
	"encoding/json"
	"fmt"

	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

type CLIMinVersionMap struct {
	MinVersion string `json:"min-version"`
}

func CheckCLIVersionIsOverMin(app *application.Avalanche, version string) error {
	minVersionBytes, err := app.Downloader.Download(constants.CLIMinVersionURL)
	if err != nil {
		return err
	}

	var parsedMinVersion CLIMinVersionMap
	if err = json.Unmarshal(minVersionBytes, &parsedMinVersion); err != nil {
		return err
	}

	minVersion := parsedMinVersion.MinVersion
	versionComparison := semver.Compare(version, minVersion)
	fmt.Printf("minVersion %s \n", minVersion)
	fmt.Printf("currentversion %s \n", version)
	if versionComparison == -1 {
		return fmt.Errorf("CLI version is required to be at least %s, current CLI version is %s, please upgrade CLI by calling `curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s`", minVersion, version)
	}
	return nil
}
