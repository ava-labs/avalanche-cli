package utils

import (
	"os/user"
	"path"
)

const (
	CLIBinary = "./bin/avalanche"
	SubnetCmd = "subnet"
	baseDir   = ".avalanche-cli"
)

func GetBaseDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return path.Join(usr.HomeDir, baseDir)
}
