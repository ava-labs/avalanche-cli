// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/apm/apm"
	"github.com/ava-labs/apm/config"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	credentialsFileKey  = "credentials-file"
	adminAPIEndpointKey = "admin-api-endpoint"
)

func SetupApm(app *application.Avalanche) error {
	credentials, err := initCredentials()
	if err != nil {
		return err
	}

	fs := afero.NewOsFs()

	fmt.Println("Plugin dir", app.GetAPMPluginDir())

	err = os.MkdirAll(app.GetAPMPluginDir(), constants.DefaultPerms755)
	if err != nil {
		return err
	}

	usr, err := user.Current()
	if err != nil {
		// no logger here yet
		fmt.Printf("unable to get system user %s\n", err)
		return err
	}
	apmBaseDir := filepath.Join(usr.HomeDir, constants.APMDir)

	// The New() function has a lot of prints we'd like to hide from the user,
	// so going to divert stdout to the log temporarily
	stdOutHolder := os.Stdout
	apmLog, err := os.OpenFile(app.GetAPMLog(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.DefaultPerms755)
	if err != nil {
		return err
	}
	defer apmLog.Close()
	os.Stdout = apmLog
	fmt.Println("testing log print")
	apmConfig := apm.Config{
		Directory:        apmBaseDir,
		Auth:             credentials,
		AdminAPIEndpoint: viper.GetString(adminAPIEndpointKey),
		PluginDir:        app.GetAPMPluginDir(),
		Fs:               fs,
	}
	apmInstance, err := apm.New(apmConfig)
	if err != nil {
		return err
	}
	os.Stdout = stdOutHolder
	app.Apm = apmInstance

	app.ApmDir = apmBaseDir
	return err
}

// If we need to use custom git credentials (say for private repos).
// the zero value for credentials is safe to use.
func initCredentials() (http.BasicAuth, error) {
	result := http.BasicAuth{}

	if viper.IsSet(credentialsFileKey) {
		credentials := &config.Credential{}

		bytes, err := os.ReadFile(viper.GetString(credentialsFileKey))
		if err != nil {
			return result, err
		}
		if err := yaml.Unmarshal(bytes, credentials); err != nil {
			return result, err
		}

		result.Username = credentials.Username
		result.Password = credentials.Password
	}

	return result, nil
}
