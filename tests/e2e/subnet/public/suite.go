// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName   = "e2eSubnetTest"
	genesisPath  = "tests/e2e/assets/test_genesis.json"
	controlKeys  = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testKey      = "tests/e2e/assets/ewoq_key.pk"
	keyName      = "key1"
	LocalNode1ID = "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
	LocalNode2ID = "NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ"
	LocalNode3ID = "NodeID-NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN"
	LocalNode4ID = "NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu"
	LocalNode5ID = "NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5"
)

var _ = ginkgo.Describe("[Public Subnet]", func() {

	ginkgo.It("initialize fuji mock env", func() {
		// fuji mock
		_ = commands.StartNetwork()
		os.Setenv(constants.DeployPublickyLocalMockEnvVar, "true")
		// key
		_ = utils.DeleteKey(keyName)
		output, err := commands.CreateKeyFromPath(keyName, testKey)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		// subnet config
		_ = utils.DeleteConfigs(subnetName)
		commands.CreateSubnetConfig(subnetName, genesisPath)
	})

	ginkgo.It("deploy a subnet to fuji", func() {
		_ = commands.DeploySubnetPublicly(subnetName, keyName, controlKeys)
	})

	ginkgo.It("finalize fuji mock env", func() {
		commands.DeleteSubnetConfig(subnetName)
		err := utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.CleanNetwork()
		os.Unsetenv(constants.DeployPublickyLocalMockEnvVar)
	})
})
