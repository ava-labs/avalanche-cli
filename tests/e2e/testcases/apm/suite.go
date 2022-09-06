// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apm

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnet1 = "wagmi"
	subnet2 = "spaces"
	vmid1   = "srEXiWaHuhNyGwPUi444Tu47ZEDwxTWrbQiuD7FmgSAQ6X7Dy"
	vmid2   = "sqja3uK17MJxfC7AN8nGadBw9JK5BcrsNwNynsqP5Gih8M5Bm"

	testRepo = "https://github.com/ava-labs/test-subnet-configs"
)

var _ = ginkgo.Describe("[Local Subnet]", func() {
	ginkgo.BeforeEach(func() {
		// TODO this is a bit coarse, but I'm not sure a better solution is possible
		// without modifications to the APM.
		// More details: https://github.com/ava-labs/avalanche-cli/issues/244
		utils.RemoveAPMRepo()
	})

	ginkgo.AfterEach(func() {
		err := utils.DeleteConfigs(subnet1)
		if err != nil {
			fmt.Println("Clean network error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(subnet2)
		if err != nil {
			fmt.Println("Delete config error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		utils.DeleteAPMBin(vmid1)
		utils.DeleteAPMBin(vmid2)
		// TODO same as above
		utils.RemoveAPMRepo()
	})

	ginkgo.It("can import from avalanche-core", func() {
		repo := "ava-labs/avalanche-plugins-core"
		commands.ImportSubnetConfig(repo, subnet1)
	})

	ginkgo.It("can import from url", func() {
		branch := "master"
		commands.ImportSubnetConfigFromURL(testRepo, branch, subnet2)
	})
})
