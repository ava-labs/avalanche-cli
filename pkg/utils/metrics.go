// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/application"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/dukex/mixpanel"
	"github.com/spf13/cobra"
)

// mixpanelToken value is set at build and install scripts using ldflags
var mixpanelToken = ""

func GetCLIVersion() string {
	wdPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	versionPath := filepath.Join(wdPath, "VERSION")
	content, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	return string(content)
}

func PrintMetricsOptOutPrompt() {
	ux.Logger.PrintToUser("Ava Labs aggregates collected data to identify patterns of usage to identify common " +
		"issues and improve the experience of Avalanche-CLI. Avalanche-CLI does not collect any private or " +
		"personal data.")
	ux.Logger.PrintToUser("You can disable data collection with `avalanche config metrics disable` command. " +
		"You can also read our privacy statement <https://www.avalabs.org/privacy-policy> to learn more.\n")
}

func saveMetricsConfig(app *application.Avalanche, metricsEnabled bool) {
	config := models.Config{MetricsEnabled: metricsEnabled}
	jsonBytes, _ := json.Marshal(&config)
	_ = app.WriteConfigFile(jsonBytes)
}

func HandleUserMetricsPreference(app *application.Avalanche) error {
	PrintMetricsOptOutPrompt()
	txt := "Press [Enter] to opt-in, or opt out by choosing 'No'"
	yes, err := app.Prompt.CaptureYesNo(txt)
	if err != nil {
		return err
	}
	if !yes {
		ux.Logger.PrintToUser("Avalanche CLI usage metrics will not be collected")
	} else {
		ux.Logger.PrintToUser("Thank you for opting in Avalanche CLI usage metrics collection")
	}
	saveMetricsConfig(app, yes)
	return nil
}

func userIsOptedIn(app *application.Avalanche) bool {
	// if config file is not found or unable to be read, will return false (user is not opted in)
	config, err := app.LoadConfig()
	if err != nil {
		return false
	}
	return config.MetricsEnabled
}

func HandleTracking(cmd *cobra.Command, app *application.Avalanche, flags map[string]string) {
	if userIsOptedIn(app) {
		TrackMetrics(cmd, flags)
	}
}

func TrackMetrics(command *cobra.Command, flags map[string]string) {
	if mixpanelToken == "" || os.Getenv("RUN_E2E") != "" {
		return
	}
	client := mixpanel.New(mixpanelToken, "")
	usr, _ := user.Current() // use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])
	mixPanelProperties := make(map[string]any)
	mixPanelProperties["command"] = command.CommandPath()
	mixPanelProperties["version"] = GetCLIVersion()
	mixPanelProperties["os"] = runtime.GOOS

	for propertyKey, propertyValue := range flags {
		mixPanelProperties[propertyKey] = propertyValue
	}
	_ = client.Track(userID, "cli-command", &mixpanel.Event{
		IP:         "0",
		Properties: mixPanelProperties,
	})
}
