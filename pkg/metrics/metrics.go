// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/posthog/posthog-go"
)

// telemetryToken value is set at build and install scripts using ldflags
var (
	telemetryToken    = ""
	telemetryInstance = "https://app.posthog.com"
	sent              = false
)

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

func HandleTracking(
	app *application.Avalanche,
	flags map[string]string,
	err error,
) {
	if sent {
		// avoid sending duplicate information for special commands with more info
		return
	}
	sent = true
	if app.Cmd == nil {
		// command called with no arguments at all
		return
	}
	if notInitialized(app) {
		if err := app.Conf.SetConfigValue(constants.ConfigMetricsEnabledKey, true); err != nil {
			ux.Logger.PrintToUser(logging.Red.Wrap("failure initializing metrics default: %s"), err)
		}
		_ = getMetricsUserID(app)
	}
	if !userIsOptedIn(app) {
		return
	}
	if !app.Cmd.HasSubCommands() && CheckCommandIsNotCompletion(app.Cmd.CommandPath()) {
		trackMetrics(app, flags, err)
	}
}

func CheckCommandIsNotCompletion(commandPath string) bool {
	result := strings.Fields(commandPath)
	if len(result) >= 2 && result[1] == "completion" {
		return false
	}
	return true
}

func trackMetrics(app *application.Avalanche, flags map[string]string, cmdErr error) {
	if telemetryToken == "" {
		telemetryToken = os.Getenv(constants.MetricsAPITokenEnvVarName)
	}
	if telemetryToken == "" && !utils.IsE2E() {
		app.Log.Warn("no token is configured for sending metrics")
	}
	if telemetryToken == "" || utils.IsE2E() {
		return
	}
	client, err := posthog.NewWithConfig(telemetryToken, posthog.Config{Endpoint: telemetryInstance})
	if err != nil {
		app.Log.Warn(fmt.Sprintf("failure creating metrics client: %s", err))
	}

	version := app.GetVersion()

	userID := getMetricsUserID(app)

	telemetryProperties := make(map[string]interface{})
	telemetryProperties["command"] = app.Cmd.CommandPath()
	telemetryProperties["cli_version"] = version
	telemetryProperties["os"] = runtime.GOOS
	telemetryProperties["was_successful"] = true
	telemetryProperties["error_msg"] = ""
	if cmdErr != nil {
		telemetryProperties["was_successful"] = false
		telemetryProperties["error_msg"] = cmdErr.Error()
	}
	telemetryProperties["environment"] = "local"
	if utils.InsideCodespace() {
		telemetryProperties["environment"] = "codespace"
	}
	for propertyKey, propertyValue := range flags {
		telemetryProperties[propertyKey] = propertyValue
	}
	event := posthog.Capture{
		DistinctId: userID,
		Event:      "cli-command",
		Properties: telemetryProperties,
	}
	if err := client.Enqueue(event); err != nil {
		app.Log.Warn(fmt.Sprintf("failure sending metrics %#v: %s", telemetryProperties, err))
	}
	if err := client.Close(); err != nil {
		app.Log.Warn(fmt.Sprintf("failure closing metrics client %#v: %s", telemetryProperties, err))
	}
}
