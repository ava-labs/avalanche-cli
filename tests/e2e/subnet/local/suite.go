// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/spacesvm/chain"
	"github.com/ava-labs/spacesvm/client"
	"github.com/ethereum/go-ethereum/crypto"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName       = "e2eSubnetTest"
	secondSubnetName = "e2eSecondSubnetTest"
	testKey          = "tests/e2e/assets/ewoq_key.pk"
)

var _ = ginkgo.Describe("[Local Subnet]", func() {
	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		err := utils.DeleteConfigs(subnetName)
		if err != nil {
			fmt.Println("Clean network error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		err = utils.DeleteConfigs(secondSubnetName)
		if err != nil {
			fmt.Println("Delete config error:", err)
		}
		gomega.Expect(err).Should(gomega.BeNil())
	})

	/*
		ginkgo.It("can deploy a custom vm subnet to local", func() {
			customVMPath, err := utils.DownloadCustomVMBin()
			gomega.Expect(err).Should(gomega.BeNil())
			commands.CreateCustomVMConfig(subnetName, utils.SubnetEvmGenesisPath, customVMPath)
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

			err = utils.RunHardhatTests(utils.BaseTest)
			gomega.Expect(err).Should(gomega.BeNil())

			commands.DeleteSubnetConfig(subnetName)
		})

		ginkgo.It("can deploy a SubnetEvm subnet to local", func() {
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

			err = utils.RunHardhatTests(utils.BaseTest)
			gomega.Expect(err).Should(gomega.BeNil())

			commands.DeleteSubnetConfig(subnetName)
		})
	*/

	ginkgo.It("can deploy a SpacesVM subnet to local", func() {
		commands.CreateSpacesVMConfig(subnetName, utils.SpacesVMGenesisPath)
		deployOutput := commands.DeploySubnetLocally(subnetName)
		rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
		if err != nil {
			fmt.Println(deployOutput)
		}
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(rpcs).Should(gomega.HaveLen(1))
		rpc := rpcs[0]

		cli := client.New(strings.ReplaceAll(rpc, "/rpc", ""), constants.RequestTimeout)

		// get genesis
		_, err = cli.Genesis(context.Background())
		gomega.Expect(err).Should(gomega.BeNil())

		// ping
		ok, err := cli.Ping(context.Background())
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(ok).Should(gomega.BeTrue())

		// network info
		networkID, _, chainID, err := cli.Network(context.Background())
		gomega.Expect(networkID).Should(gomega.Equal(uint32(constants.LocalNetworkID)))
		gomega.Expect(chainID).ShouldNot(gomega.Equal(ids.Empty))
		gomega.Expect(err).Should(gomega.BeNil())

		// get signing key
		privHexBytes, err := os.ReadFile(testKey)
		gomega.Expect(err).Should(gomega.BeNil())
		priv, err := crypto.HexToECDSA(strings.TrimSpace(string(privHexBytes)))
		gomega.Expect(err).Should(gomega.BeNil())
		sender := crypto.PubkeyToAddress(priv.PublicKey)

		// reserve space
		space := "clispace"
		claimTx := &chain.ClaimTx{
			BaseTx: &chain.BaseTx{},
			Space:  space,
		}
		claimed, err := cli.Claimed(context.Background(), space)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(claimed).Should(gomega.BeFalse())
		ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
		_, _, err = client.SignIssueRawTx(
			ctx,
			cli,
			claimTx,
			priv,
			client.WithPollTx(),
			client.WithInfo(space),
		)
		cancel()
		gomega.Expect(err).Should(gomega.BeNil())
		claimed, err = cli.Claimed(context.Background(), space)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(claimed).Should(gomega.BeTrue())
		pf, _, err := cli.Info(context.Background(), space)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(pf.Units).To(gomega.Equal(uint64(100)))
		gomega.Expect(pf.Owner).To(gomega.Equal(sender))

		// set key/val pair
		k, v := "key", []byte("value")
		setTx := &chain.SetTx{
			BaseTx: &chain.BaseTx{},
			Space:  space,
			Key:    k,
			Value:  v,
		}
		ctx, cancel = context.WithTimeout(context.Background(), constants.RequestTimeout)
		_, _, err = client.SignIssueRawTx(
			ctx,
			cli,
			setTx,
			priv,
			client.WithPollTx(),
			client.WithInfo(space),
		)
		cancel()
		gomega.Expect(err).To(gomega.BeNil())

		// check key/val pair
		_, rv, _, err := cli.Resolve(context.Background(), space+"/"+k)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(rv).To(gomega.Equal(v))

		commands.DeleteSubnetConfig(subnetName)
	})

	/*
		ginkgo.It("can't deploy the same subnet twice to local", func() {
			commands.CreateSubnetEvmConfig(subnetName, utils.SubnetEvmGenesisPath)

			deployOutput := commands.DeploySubnetLocally(subnetName)
			rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
			if err != nil {
				fmt.Println(deployOutput)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rpcs).Should(gomega.HaveLen(1))

			deployOutput = commands.DeploySubnetLocally(subnetName)
			rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
			if err == nil {
				fmt.Println(deployOutput)
			}
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(rpcs).Should(gomega.HaveLen(0))
			gomega.Expect(deployOutput).Should(gomega.ContainSubstring("has already been deployed"))
		})

		ginkgo.It("can deploy multiple subnets to local", func() {
			commands.CreateSubnetEvmConfig(subnetName, utils.SubnetEvmGenesisPath)
			commands.CreateSubnetEvmConfig(secondSubnetName, utils.SubnetEvmGenesisPath)

			deployOutput := commands.DeploySubnetLocally(subnetName)
			rpcs, err := utils.ParseRPCsFromOutput(deployOutput)
			if err != nil {
				fmt.Println(deployOutput)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rpcs).Should(gomega.HaveLen(1))

			deployOutput = commands.DeploySubnetLocally(secondSubnetName)
			rpcs, err = utils.ParseRPCsFromOutput(deployOutput)
			if err != nil {
				fmt.Println(deployOutput)
			}
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(rpcs).Should(gomega.HaveLen(2))

			err = utils.SetHardhatRPC(rpcs[0])
			gomega.Expect(err).Should(gomega.BeNil())

			err = utils.RunHardhatTests(utils.BaseTest)
			gomega.Expect(err).Should(gomega.BeNil())

			err = utils.SetHardhatRPC(rpcs[1])
			gomega.Expect(err).Should(gomega.BeNil())

			err = utils.RunHardhatTests(utils.BaseTest)
			gomega.Expect(err).Should(gomega.BeNil())

			commands.DeleteSubnetConfig(subnetName)
			commands.DeleteSubnetConfig(secondSubnetName)
		})
	*/
})
