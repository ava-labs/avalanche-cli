// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
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
