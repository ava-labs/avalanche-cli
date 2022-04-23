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

var BaseDir string

func init() {
	usr, _ := user.Current()
	newpath := filepath.Join(usr.HomeDir, ".avalanche-cli")
	BaseDir = newpath
	err := os.MkdirAll(BaseDir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func main() {
	cmd.Execute()
}
