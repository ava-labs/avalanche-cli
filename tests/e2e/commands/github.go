// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"context"
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/onsi/gomega"
)

const avalanchegoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"

func GetLatestAvagoVersionFromGithub() string {
	response, err := utils.MakeGetRequest(context.Background(), avalanchegoReleaseURL)
	gomega.Expect(err).Should(gomega.BeNil())
	var releaseInfo map[string]interface{}
	err = json.Unmarshal(response, &releaseInfo)
	gomega.Expect(err).Should(gomega.BeNil())
	tagName, ok := releaseInfo["tag_name"].(string)
	gomega.Expect(ok).Should(gomega.BeTrue())
	return tagName
}
