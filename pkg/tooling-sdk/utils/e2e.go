// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

const (
	E2EListenPrefix = "192.168.223"
)

// Config holds the information needed for the template
type Config struct {
	IPs           []string
	UbuntuVersion string
	NetworkPrefix string
	SSHPubKey     string
	E2ESuffixList []string
}

// IsE2E checks if the environment variable "RUN_E2E" is set and returns true if it is, false otherwise.
func IsE2E() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return os.Getenv("RUN_E2E") == "true" || currentUser.Username == "runner"
}

// E2EDocker checks if docker and docker-compose are available.
func E2EDocker() bool {
	cmd := exec.Command("docker", "--version")
	cmd.Env = os.Environ()
	err := cmd.Run()
	return err == nil
}

// E2EConvertIP maps an IP address to an E2E IP address.
func E2EConvertIP(ip string) string {
	if suffix := E2ESuffix(ip); suffix != "" {
		return fmt.Sprintf("%s.10%s", E2EListenPrefix, suffix)
	} else {
		return ""
	}
}

func E2ESuffix(ip string) string {
	addressBits := strings.Split(ip, ".")
	if len(addressBits) != 4 {
		return ""
	} else {
		return addressBits[3]
	}
}

func RemoveLineCleanChars(s string) string {
	re := regexp.MustCompile(`\r\x1b\[K`)
	return re.ReplaceAllString(s, "")
}
