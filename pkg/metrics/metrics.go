// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

import (
	"os"
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

func getMetricsUserID(app *application.Avalanche) string {
	if !app.Conf.ConfigFileExists() || !app.Conf.ConfigValueIsSet(constants.ConfigMetricsUserIDKey) {
		userID := utils.RandomString(20)
		if err := app.Conf.SetConfigValue(constants.ConfigMetricsUserIDKey, userID); err != nil {
			ux.Logger.PrintToUser(logging.Red.Wrap("failure initializing metrics id: %s"), err)
		}
		return userID
	}
	return app.Conf.GetConfigStringValue(constants.ConfigMetricsUserIDKey)
}

func notInitialized(app *application.Avalanche) bool {
	return !app.Conf.ConfigFileExists() || !app.Conf.ConfigValueIsSet(constants.ConfigMetricsEnabledKey)
}

func userIsOptedIn(app *application.Avalanche) bool {
	return app.Conf.ConfigFileExists() && app.Conf.GetConfigBoolValue(constants.ConfigMetricsEnabledKey)
}

func HandleTracking(cmd *cobra.Command, commandPath string, app *application.Avalanche, flags map[string]string) {
	if notInitialized(app) {
		if err := app.Conf.SetConfigValue(constants.ConfigMetricsEnabledKey, true); err != nil {
			ux.Logger.PrintToUser(logging.Red.Wrap("failure initializing metrics default: %s"), err)
		}
		_ = getMetricsUserID(app)
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

	userID := getMetricsUserID(app)

	telemetryProperties := make(map[string]interface{})
	telemetryProperties["command"] = commandPath
	telemetryProperties["version"] = version
	telemetryProperties["os"] = runtime.GOOS
	insideCodespace := utils.InsideCodespace()
	telemetryProperties["insideCodespace"] = insideCodespace
	if insideCodespace {
		codespaceName := os.Getenv(constants.CodespaceNameEnvVar)
		telemetryProperties["codespace"] = codespaceName
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
