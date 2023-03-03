// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

const noLimitNumberOfMatches = -1

var (
	filters = []string{
		"failing health check",
		"DEBUG",
		"INFO",
	}
	// It was initially suggested to also look for all warn messages,
	// however this results in too much stuff printed to the screen.
	// For documentation reasons though leaving the according regex here
	// so it can easily be re-enabled if wished.
	// errorRegEx := regexp.MustCompile(`(?im)(^.*error.*|.*warn.*)`)
	errorRegEx = regexp.MustCompile(`(?im)(^.*error.*)`)
)

// FindErrorLogs is a utility function, we will NOT do error handling,
// as this is supposed to be called during error handling itself
// we don't want to make it even more complex
func FindErrorLogs(rootDirs ...string) {
	alreadyNotified := false
	foundErrors := false

	for _, rootDir := range rootDirs {
		if _, err := os.Stat(rootDir); err != nil {
			return
		}

		_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				if !strings.HasSuffix(d.Name(), "log") {
					return nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				occurrences := errorRegEx.FindAllString(string(content), noLimitNumberOfMatches)
				thisFileNotified := false
				for _, o := range occurrences {
					// first apply all filters
					skip := false
					for _, f := range filters {
						if strings.Contains(o, f) {
							skip = true
						}
					}
					if skip {
						continue
					}
					if !alreadyNotified {
						fmt.Println()
						ux.Logger.PrintToUser("================================= !!! ================================")
						ux.Logger.PrintToUser("Found some error strings in the logs, check these for possible causes:")
						alreadyNotified = true
						foundErrors = true
					}
					if !thisFileNotified {
						ux.Logger.PrintToUser("----------------------------------------------------------------------")
						ux.Logger.PrintToUser("-- Found error logs in file at path %s:", path)
						thisFileNotified = true
						fmt.Println()
					}
					ux.Logger.PrintToUser(o)
					ux.Logger.PrintToUser("----------------------------------------------------------------------")
					fmt.Println()
				}
			}
			return nil
		})
	}
	if foundErrors {
		ux.Logger.PrintToUser("================!!! end of errors in logs !!! ========================")
	}
}
