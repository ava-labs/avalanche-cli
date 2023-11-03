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
	"runtime"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// telemetryToken value is set at build and install scripts using ldflags
var (
	telemetryToken    = ""
	telemetryInstance = "https://app.posthog.com"
	app               *application.Avalanche
)

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
	ux.Logger.PrintToUser(
		"Avalanche-CLI (the \"software\") may collect statistical data on how the software is used on an anonymous " +
			"basis for purposes of product improvement.  This data will not (i) include any passwords, scripts, or data " +
			"files, (ii) be associated with any particular user or entity, or (iii) include any personally identifiable " +
			"information or be used to identify individuals or entities using the software.  You can disable such data " +
			"collection with `avalanche config metrics disable` command, which will result in no data being collected; " +
			"by using the software without so disabling such data collection you expressly consent to the collection of " +
			"such data.  You can also read our privacy statement <https://www.avalabs.org/privacy-policy> to learn more. \n")
	ux.Logger.PrintToUser("You can disable data collection with `avalanche config metrics disable` command. " +
		"You can also read our privacy statement <https://www.avalabs.org/privacy-policy> to learn more.\n")
}

func saveMetricsConfig(metricsEnabled bool) error {
	return app.SetConfigValue(constants.ConfigMetricsEnabled, metricsEnabled)
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
	if err = saveMetricsConfig(yes); err != nil {
		return err
	}
	return nil
}

func userIsOptedIn() bool {
	return viper.GetBool(constants.ConfigMetricsEnabled)
}

func HandleTracking(cmd *cobra.Command, flags map[string]string) {
	if userIsOptedIn() {
		if !cmd.HasSubCommands() && checkCommandIsNotCompletion(cmd) {
			TrackMetrics(cmd, flags)
		}
	}
}

func checkCommandIsNotCompletion(cmd *cobra.Command) bool {
	result := strings.Fields(cmd.CommandPath())
	if len(result) >= 2 && result[1] == "completion" {
		return false
	}
	return true
}

func TrackMetrics(command *cobra.Command, flags map[string]string) {
	if telemetryToken == "" || os.Getenv("RUN_E2E") != "" {
		return
	}

	client, _ := posthog.NewWithConfig(telemetryToken, posthog.Config{Endpoint: telemetryInstance})

	defer client.Close()

	usr, _ := user.Current() // use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])
	telemetryProperties := make(map[string]interface{})
	telemetryProperties["command"] = command.CommandPath()
	telemetryProperties["version"] = GetCLIVersion()
	telemetryProperties["os"] = runtime.GOOS
	for propertyKey, propertyValue := range flags {
		telemetryProperties[propertyKey] = propertyValue
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: userID,
		Event:      "cli-command",
		Properties: telemetryProperties,
	})
}
