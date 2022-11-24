// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName = "e2eSubnetTest"
)

var (
	mapping map[string]string
	err     error
)

var _ = ginkgo.Describe("[Network]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeAll(func() {
		app := &application.Avalanche{
			Downloader: application.NewDownloader(),
		}
		mapping, err = utils.GetVersionMapping(app)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("can stop and restart a deployed subnet", func() {
		commands.CreateSubnetEvmConfig(subnetName, utils.SubnetEvmGenesisPath)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		// Deploy greeter contract
		scriptOutput, scriptErr, err := utils.RunHardhatScript(utils.GreeterScript)
		if scriptErr != "" {
			fmt.Println(scriptOutput)
			fmt.Println(scriptErr)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.ParseGreeterAddress(scriptOutput)
		gomega.Expect(err).Should(gomega.BeNil())

		// Check greeter script before stopping
		scriptOutput, scriptErr, err = utils.RunHardhatScript(utils.GreeterCheck)
		if scriptErr != "" {
			fmt.Println(scriptOutput)
			fmt.Println(scriptErr)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		commands.StopNetwork()
		restartOutput := commands.StartNetwork()
		rpcs, err = utils.ParseRPCsFromOutput(restartOutput)
		if err != nil {
			fmt.Println(restartOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc = rpcs[0]

		err = utils.SetHardhatRPC(rpc)
		gomega.Expect(err).Should(gomega.BeNil())

		// Check greeter contract has right value
		scriptOutput, scriptErr, err = utils.RunHardhatScript(utils.GreeterCheck)
		if scriptErr != "" {
			fmt.Println(scriptOutput)
			fmt.Println(scriptErr)
		}
		gomega.Expect(err).Should(gomega.BeNil())

		commands.DeleteSubnetConfig(subnetName)
	})
})
