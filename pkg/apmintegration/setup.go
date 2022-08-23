package apmintegration

import (
	"os"

	"github.com/ava-labs/apm/apm"
	"github.com/ava-labs/apm/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	apmPathKey          = "apm-path"
	pluginPathKey       = "plugin-path"
	credentialsFileKey  = "credentials-file"
	adminAPIEndpointKey = "admin-api-endpoint"
)

func SetupApm() (*apm.APM, error) {
	credentials, err := initCredentials()
	if err != nil {
		return nil, err
	}

	fs := afero.NewOsFs()

	return apm.New(apm.Config{
		Directory:        viper.GetString(apmPathKey),
		Auth:             credentials,
		AdminAPIEndpoint: viper.GetString(adminAPIEndpointKey),
		PluginDir:        viper.GetString(pluginPathKey),
		Fs:               fs,
	})
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
