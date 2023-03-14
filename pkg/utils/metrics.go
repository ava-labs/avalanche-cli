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
var mixpanelToken = "be5c4cee5052278568d7a153430173a1"

func TrackMetrics(command *cobra.Command) {
	client := mixpanel.New(mixpanelToken, "")
	usr, _ := user.Current() //use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])

	_ = client.Track(userID, "cli-command", &mixpanel.Event{
		IP: "0",
		Properties: map[string]any{
			"command": command.CommandPath(),
		},
	})
}
