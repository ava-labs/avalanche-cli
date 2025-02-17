// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

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
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
)

// telemetryToken value is set at build and install scripts using ldflags
var (
	telemetryToken    = ""
	telemetryInstance = "https://app.posthog.com"
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

func userDoesNotOpted(app *application.Avalanche) bool {
	return !app.Conf.ConfigFileExists() || !app.Conf.ConfigValueIsSet(constants.ConfigMetricsEnabledKey)
}

func userIsOptedIn(app *application.Avalanche) bool {
	return app.Conf.ConfigFileExists() && app.Conf.GetConfigBoolValue(constants.ConfigMetricsEnabledKey)
}

func HandleTracking(cmd *cobra.Command, commandPath string, app *application.Avalanche, flags map[string]string) {
	if userDoesNotOpted(app) {
		if err := handleUserMetricsPreference(app); err != nil {
			ux.Logger.PrintToUser(logging.Red.Wrap("failure setting metrics preference: %s"), err)
			return
		}
	}
	if !userIsOptedIn(app) {
		return
	}
	if !cmd.HasSubCommands() && CheckCommandIsNotCompletion(cmd) {
		trackMetrics(app, commandPath, flags)
	}
}

func CheckCommandIsNotCompletion(cmd *cobra.Command) bool {
	result := strings.Fields(cmd.CommandPath())
	if len(result) >= 2 && result[1] == "completion" {
		return false
	}
	return true
}

func trackMetrics(app *application.Avalanche, commandPath string, flags map[string]string) {
	if telemetryToken == "" {
		telemetryToken = os.Getenv(constants.MetricsAPITokenEnvVarName)
	}
	if telemetryToken == "" || utils.IsE2E() {
		return
	}
	client, _ := posthog.NewWithConfig(telemetryToken, posthog.Config{Endpoint: telemetryInstance})

	defer client.Close()

	version := app.Version
	if version == "" {
		version = GetCLIVersion()
	}

	usr, _ := user.Current() // use empty string if err
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s", usr.Username, usr.Uid)))
	userID := base64.StdEncoding.EncodeToString(hash[:])

	telemetryProperties := make(map[string]interface{})
	telemetryProperties["command"] = commandPath
	telemetryProperties["version"] = version
	telemetryProperties["os"] = runtime.GOOS
	insideCodespace := utils.InsideCodespace()
	telemetryProperties["insideCodespace"] = insideCodespace
	if insideCodespace {
		codespaceName := os.Getenv(constants.CodespaceNameEnvVar)
		telemetryProperties["codespace"] = codespaceName
		hash := sha256.Sum256([]byte(codespaceName))
		userID = base64.StdEncoding.EncodeToString(hash[:])
	}
	for propertyKey, propertyValue := range flags {
		telemetryProperties[propertyKey] = propertyValue
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: userID,
		Event:      "cli-command",
		Properties: telemetryProperties,
	})
}

func saveMetricsConfig(app *application.Avalanche, metricsEnabled bool) error {
	return app.Conf.SetConfigValue(constants.ConfigMetricsEnabledKey, metricsEnabled)
}

func handleUserMetricsPreference(app *application.Avalanche) error {
	if utils.IsE2E() {
		return saveMetricsConfig(app, false)
	}
	ux.Logger.PrintToUser(
		"\nAvalanche-CLI (the \"software\") may collect statistical data on how the software is used on an anonymous " +
			"basis for purposes of product improvement.  This data will not (i) include any passwords, scripts, or data " +
			"files, (ii) be associated with any particular user or entity, or (iii) include any personally identifiable " +
			"information or be used to identify individuals or entities using the software.  You can disable such data " +
			"collection with `avalanche config metrics disable` command, which will result in no data being collected; " +
			"by using the software without so disabling such data collection you expressly consent to the collection of " +
			"such data.  You can also read our privacy statement <https://www.avalabs.org/privacy-policy> to learn more. \n",
	)
	ux.Logger.PrintToUser(
		"You can disable data collection with `avalanche config metrics disable` command. " +
			"You can also read our privacy statement <https://www.avalabs.org/privacy-policy> to learn more.\n",
	)
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
	if err = saveMetricsConfig(app, yes); err != nil {
		return err
	}
	return nil
}
