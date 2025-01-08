# Public Archive Downloader SDK

This Go package provides a utility to download and extract tar archives from public URLs. It's tailored for downloading Avalanche network archives but can be adapted for other use cases.


## Features

* Downloads files from predefined URLs.
* Tracks download progress and logs status updates.
* Safely unpacks .tar archives to a target directory.
* Includes security checks to prevent path traversal and manage large files.

## Usage example

```
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved
// See the file LICENSE for licensing terms.

```
package main

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/your-repo-name/publicarchive"
)

func main() {
	// Initialize the downloader
	downloader, err := publicarchive.NewDownloader(network.FujiNetwork(), logging.Debug)
	if err != nil {
		fmt.Printf("Failed to create downloader: %v\n", err)
		os.Exit(1)
	}

	// Start downloading
	if err := downloader.Download(); err != nil {
		fmt.Printf("Download failed: %v\n", err)
		os.Exit(1)
	}

	// Specify the target directory for unpacking
	targetDir := "./extracted_files"
	if err := downloader.UnpackTo(targetDir); err != nil {
		fmt.Printf("Failed to unpack archive: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Files successfully unpacked to %s\n", targetDir)
}
```
