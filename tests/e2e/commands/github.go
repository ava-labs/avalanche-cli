// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"encoding/json"
	"net/http"

	"github.com/onsi/gomega"
)

const avalanchegoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"

func GetLatestAvagoVersionFromGithub() string {
	response, err := http.Get(avalanchegoReleaseURL)
	gomega.Expect(err).Should(gomega.BeNil())
	defer response.Body.Close()
	gomega.Expect(response.StatusCode).Should(gomega.BeEquivalentTo(http.StatusOK))
	var releaseInfo map[string]interface{}
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&releaseInfo)
	gomega.Expect(err).Should(gomega.BeNil())
	tagName, ok := releaseInfo["tag_name"].(string)
	gomega.Expect(ok).Should(gomega.BeTrue())
	return tagName
}
