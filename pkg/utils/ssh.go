package utils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func GetSSHConnectionString(publicIP, certFilePath string) string {
	return fmt.Sprintf("ssh %s %s@%s -i %s", constants.AnsibleSSHParams, constants.AnsibleSSHUser, publicIP, certFilePath)
}
