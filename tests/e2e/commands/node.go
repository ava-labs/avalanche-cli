// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/onsi/gomega"
)

const (
	e2eKeyPairName = "runner-avalanche-cli-keypair"
	ExpectSuccess  = true
)

func NodeProvision(network, version string, numNodes int, separateMonitoring bool, numAPINodes int, expectSuccess bool) string {
	home, err := os.UserHomeDir()
	gomega.Expect(err).Should(gomega.BeNil())
	_, err = os.Open(filepath.Join(home, ".ssh", e2eKeyPairName))
	gomega.Expect(err).Should(gomega.BeNil())
	_, err = os.Open(filepath.Join(home, ".ssh", e2eKeyPairName+".pub"))
	gomega.Expect(err).Should(gomega.BeNil())
	cmdVersion := "--latest-avalanchego-version=true"
	if version != "latest" && version != "" {
		cmdVersion = "--custom-avalanchego-version=" + version
	}
	cmdAPI := ""
	if numAPINodes > 0 {
		cmdAPI = "--num-apis=" + strconv.Itoa(numAPINodes)
	}
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"provision",
		constants.E2EClusterName,
		"--use-static-ip=false",
		cmdVersion,
		"--enable-monitoring="+strconv.FormatBool(separateMonitoring),
		"--region=local",
		"--num-validators="+strconv.Itoa(numNodes),
		"--"+network,
		"--node-type=docker",
	)
	if cmdAPI != "" {
		cmd.Args = append(cmd.Args, cmdAPI)
	}
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.CombinedOutput()
	fmt.Println("---------------->")
	fmt.Println(string(output))
	fmt.Println(err)
	fmt.Println("---------------->")
	if expectSuccess {
		gomega.Expect(err).Should(gomega.BeNil())
	} else {
		gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
	}

	return string(output)
}

func NodeDevnet(version string, numNodes int, numAPINodes int) string {
	cmdVersion := "--latest-avalanchego-version=true"
	if version != "latest" && version != "" {
		cmdVersion = "--custom-avalanchego-version=" + version
	}
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"provision",
		constants.E2EClusterName,
		"--use-static-ip=false",
		"--enable-monitoring=false",
		cmdVersion,
		"--region=local",
		"--num-validators="+strconv.Itoa(numNodes),
		"--num-apis="+strconv.Itoa(numAPINodes),
		"--devnet",
		"--node-type=docker",
	)
	return runCmd(cmd, ExpectSuccess)
}

func NodeStatus() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"status",
		constants.E2EClusterName,
	)
	return runCmd(cmd, ExpectSuccess)
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
	multilineText := runCmd(cmd, ExpectSuccess)
	// filter out additional output
	pattern := `\[Node docker.*?\(NodeID-[^\s]+\)`
	re := regexp.MustCompile(pattern)

	lines := strings.Split(multilineText, "\n")
	var output []string
	for _, line := range lines {
		if re.MatchString(line) {
			continue
		} else {
			output = append(output, line)
		}
	}
	return strings.Join(output, "\n")
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
	return runCmd(cmd, ExpectSuccess)
}

func NodeWhitelistSSH(sshPubKey string) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"whitelist",
		constants.E2EClusterName,
		"--ssh",
		"\""+sshPubKey+"\"",
	)
	return runCmd(cmd, ExpectSuccess)
}

func NodeUpgrade() string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"upgrade",
		constants.E2EClusterName,
	)
	return runCmd(cmd, ExpectSuccess)
}

func NodeExport(filename string, withSecrets bool) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"export",
		constants.E2EClusterName,
		"--file",
		filename,
	)
	if withSecrets {
		cmd.Args = append(cmd.Args, "--include-secrets")
	}
	return runCmd(cmd, ExpectSuccess)
}

func NodeImport(filename string, clusterName string) string {
	/* #nosec G204 */
	cmd := exec.Command(
		CLIBinary,
		"node",
		"import",
		clusterName,
		"--file",
		filename,
	)
	return runCmd(cmd, ExpectSuccess)
}

type StaticConfig struct {
	Targets []string `yaml:"targets"`
}
type ScrapeConfig struct {
	JobName       string         `yaml:"job_name"`
	StaticConfigs []StaticConfig `yaml:"static_configs"`
}
type PrometheusConfig struct {
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`
}

// ParsePrometheusYamlConfig parses prometheus config YAML file installed in separate monitoring
// host in /etc/prometheus/prometheus.yml
func ParsePrometheusYamlConfig(filePath string) PrometheusConfig {
	data, err := os.ReadFile(filePath)
	gomega.Expect(err).Should(gomega.BeNil())
	var prometheusConfig PrometheusConfig
	err = yaml.Unmarshal(data, &prometheusConfig)
	gomega.Expect(err).Should(gomega.BeNil())
	return prometheusConfig
}

func runCmd(cmd *exec.Cmd, expectSuccess bool) string { //nolint:all
	cmd.Env = os.Environ()
	fmt.Println("About to run: " + cmd.String())
	output, err := cmd.CombinedOutput()
	fmt.Println("---------------->")
	fmt.Println(string(output))
	fmt.Println(err)
	fmt.Println("---------------->")
	if expectSuccess {
		gomega.Expect(err).Should(gomega.BeNil())
	} else {
		gomega.Expect(err).Should(gomega.Not(gomega.BeNil()))
	}
	return string(output)
}
