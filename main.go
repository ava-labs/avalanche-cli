/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/cmd"
)

func init() {
	usr, _ := user.Current()
	basePath := filepath.Join(usr.HomeDir, cmd.BaseDir)
	err := os.MkdirAll(basePath, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func main() {
	cmd.Execute()
}
