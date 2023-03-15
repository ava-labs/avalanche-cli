// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/dukex/mixpanel"
	"github.com/spf13/cobra"
)

// mixpanelToken value is set at build and install scripts using ldflags
var mixpanelToken = ""

func GetCLIVersion() (string, error) {
	wdPath, err := os.Getwd()
	if err != nil {
		return "", err
	}
	versionPath := filepath.Join(wdPath, "VERSION")
	content, err := os.ReadFile(versionPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func TrackMetrics(command *cobra.Command) {
	if mixpanelToken == "" {
		return
	}
	client := mixpanel.New(mixpanelToken, "")
	usr, _ := user.Current() // use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])

	_ = client.Track(userID, "cli-command", &mixpanel.Event{
		IP: "0",
		Properties: map[string]any{
			"command": command.CommandPath(),
			"version": GetCLIVersion(),
		},
	})
}
