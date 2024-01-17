// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
)

const e2eKeyPairName = "runner-avalanche-cli-keypair"

func createKeyPair() {
	home, err := os.UserHomeDir()
	gomega.Expect(err).Should(gomega.BeNil())
	privateKeyPath := filepath.Join(home, ".ssh", e2eKeyPairName)
	pubKeyPath := filepath.Join(home, ".ssh", e2eKeyPairName+".pub")
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	gomega.Expect(err).Should(gomega.BeNil())
	privateKeyFile, err := os.Create(privateKeyPath)
	gomega.Expect(err).Should(gomega.BeNil())
	defer privateKeyFile.Close()
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	err = pem.Encode(privateKeyFile, privateKeyPEM)
	gomega.Expect(err).Should(gomega.BeNil())
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	gomega.Expect(err).Should(gomega.BeNil())
	err = os.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0600)
	gomega.Expect(err).Should(gomega.BeNil())
}

func NodeCreate(network string, numNodes int) string {
	createKeyPair()
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"create",
		constants.E2EClusterName,
		"--use-static-ip=false",
		"--latest-avalanchego-version=true",
		"--region=local",
		"--num-nodes="+strconv.Itoa(numNodes),
		"--"+network,
		"--node-type=docker",
	)
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.Output()
	fmt.Println(string(output))
	fmt.Println(err)
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func NodeDevnet(numNodes int) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"create",
		constants.E2EClusterName,
		"--use-static-ip=false",
		"--latest-avalanchego-version=true",
		"--region=local",
		"--num-nodes="+strconv.Itoa(numNodes),
		"--devnet",
		"--node-type=docker",
	)
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	fmt.Println("---------------->")
	fmt.Println(string(output))
	fmt.Println(err)
	fmt.Println("---------------->")
	return string(output)
}

func NodeStatus() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"status",
		constants.E2EClusterName,
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func NodeSSH(name, command string) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"ssh",
		name,
		command,
	)
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}

func ConfigMetrics() {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"config",
		"metrics",
		"disable",
	)
	_, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
}

func NodeList() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"list",
	)
	output, err := cmd.Output()
	gomega.Expect(err).Should(gomega.BeNil())
	return string(output)
}
