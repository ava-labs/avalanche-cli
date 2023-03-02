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

// FindErrorLogs is a utility function, we will NOT do error handling,
// as this is supposed to be called during error handling itself
// we don't want to make it even more complex
func FindErrorLogs(rootDirs ...string) {
	errorRegEx := regexp.MustCompile(`(?im)(^.*error.*|.*warn.*)`)
	alreadyNotified := false

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
				if len(occurrences) > 0 {
					if !alreadyNotified {
						fmt.Println()
						ux.Logger.PrintToUser("================================= !!! ================================")
						ux.Logger.PrintToUser("Found some error strings in the logs, check these for possible causes:")
						alreadyNotified = true
					}
					fmt.Println()
					ux.Logger.PrintToUser("-- Found error logs in file at path %s:", path)
					for _, o := range occurrences {
						ux.Logger.PrintToUser(o)
					}
					ux.Logger.PrintToUser("================================= !!! ================================")
					fmt.Println()
				}
			}
			return nil
		})
	}
}
