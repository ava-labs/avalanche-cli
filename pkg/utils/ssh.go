package utils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetSshConnectionString(publicIP, certFilePath string) string {
	return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSshParams, constants.AnsibleSshUser, publicIP, certFilePath)
}
