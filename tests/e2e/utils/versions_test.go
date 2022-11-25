// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func TestVersions(t *testing.T) {
	app := &application.Avalanche{
		Downloader: application.NewDownloader(),
	}
	mapping, err := GetVersionMapping(app)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(mapping)
}

func TestGetVersions(t *testing.T) {
	app := &application.Avalanche{
		Downloader: application.NewDownloader(),
	}
	versions, mapping, err := getVersions(constants.SubnetEVMRPCCompatibilityURL, app)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(versions)
	fmt.Println(mapping)
}
