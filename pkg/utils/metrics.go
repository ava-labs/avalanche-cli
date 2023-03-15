// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os/user"

	"github.com/dukex/mixpanel"
	"github.com/spf13/cobra"
)

// mixpanelToken value is set at build and install scripts using ldflags
var MIXPANEL_PROJECT_TOKEN = ""

func TrackMetrics(command *cobra.Command, version string) {
	if MIXPANEL_PROJECT_TOKEN == "" {
		return
	}
	client := mixpanel.New(MIXPANEL_PROJECT_TOKEN, "")
	usr, _ := user.Current() // use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])

	_ = client.Track(userID, "cli-command", &mixpanel.Event{
		IP: "0",
		Properties: map[string]any{
			"command": command.CommandPath(),
			"version": version,
		},
	})
}
