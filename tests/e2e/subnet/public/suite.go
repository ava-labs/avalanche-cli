// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName  = "e2eSubnetTest"
	genesisPath = "tests/e2e/assets/test_genesis.json"
	controlKeys = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	testKey     = "tests/e2e/assets/ewoq_key.pk"
	keyName     = "key1"
)

var _ = ginkgo.Describe("[Public Subnet]", func() {
	ginkgo.It("can deploy a subnet to fake fuji", func() {
		_ = utils.DeleteConfigs(subnetName)
		_ = utils.DeleteKey(keyName)

		commands.CreateSubnetConfig(subnetName, genesisPath)
		output, err := commands.CreateKeyFromPath(keyName, testKey)
		if err != nil {
			fmt.Println(output)
			utils.PrintStdErr(err)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		_ = commands.StartNetwork()

		_ = commands.DeploySubnetPubliclyLocalMock(subnetName, keyName, controlKeys)

		err = utils.DeleteKey(keyName)
		gomega.Expect(err).Should(gomega.BeNil())
		commands.DeleteSubnetConfig(subnetName)
		commands.CleanNetwork()
	})
})
