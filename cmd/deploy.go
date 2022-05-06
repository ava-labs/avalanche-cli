/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/pkg/reexec"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your subnet to a network",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

const (
	latestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	binaryServerURL       = "http://3.84.91.164:8998"
	serverRun             = "/tmp/gRPCserver.run"
	binDir                = "bin"
)

var (
	deployLocal *bool
	force       *bool
)

func init() {
	subnetCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	deployLocal = deployCmd.Flags().BoolP("local", "l", false, "Deploy subnet locally")
	force = deployCmd.Flags().BoolP("force", "f", false, "Deploy without asking for confirmation")
}

func getChainsInSubnet(subnetName string) ([]string, error) {
	usr, _ := user.Current()
	mainDir := filepath.Join(usr.HomeDir, BaseDir)
	files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		return []string{}, err
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(mainDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return []string{}, err
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return []string{}, err
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

func deploySubnet(cmd *cobra.Command, args []string) error {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return err
	}

	if len(chains) == 0 {
		return fmt.Errorf("Invalid subnet: %s", args[0])
	}

	var network models.Network
	if *deployLocal {
		network = models.Local
	} else {
		networkStr, err := prompts.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	fmt.Println("Deploying", chains, "to", network.String())
	// TODO
	switch network {
	case models.Local:
		// WRITE CODE HERE
		fmt.Println("Deploy local")
		return deployToLocalNetwork(chains[0])
	default:
		return errors.New("Not implemented")
	}
}

func deployToLocalNetwork(chain string) error {
	isRunning, err := IsServerProcessRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		fmt.Println("gRPC server is not running")
		if err := startServerProcess(); err != nil {
			return err
		}
	}
	return doDeploy(chain)
}

func avagoExists(binDir string) (bool, string, error) {
	// TODO this still has loads of potential pit falls
	// Should prob check for existing binary and plugin dir too
	match, err := filepath.Glob(filepath.Join(binDir, "avalanchego") + "*")
	if err != nil {
		return false, "", err
	}
	var latest string
	switch len(match) {
	case 0:
		return false, "", nil
	case 1:
		latest = match[0]
	default:
		semVers := make(semver.Versions, len(match))
		for i, v := range match {
			base := filepath.Base(v)
			semVers[i] = semver.New(base[1:])
		}

		sort.Sort(sort.Reverse(semVers))
		choose := fmt.Sprintf("v%s", semVers[0])
		for _, m := range match {
			if strings.Contains(m, choose) {
				latest = m
				break
			}
		}
	}
	return true, latest, nil
}

func setupLocalEnv() (string, error) {
	usr, _ := user.Current()
	binDir := filepath.Join(usr.HomeDir, BaseDir, binDir)

	exists, latest, err := avagoExists(binDir)
	if err != nil {
		return "", err
	}
	if exists {
		fmt.Println("local avalanchego found. skipping installation")
		return latest, nil
	}

	fmt.Println("installing latest avalanchego version...")

	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	resp, err := http.Get(latestAvagoReleaseURL)
	if err != nil {
		return "", err
	}

	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var jsonStr map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonStr); err != nil {
		return "", err
	}

	version := jsonStr["tag_name"].(string)
	if version == "" || version[0] != 'v' {
		return "", fmt.Errorf("invalid version string: %s", version)
	}
	resp.Body.Close()

	fmt.Printf("latest avalanchego version is: %s\n", version)

	arch := runtime.GOARCH
	goos := runtime.GOOS
	avalanchegoURL := fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-linux-%s-%s.tar.gz", version, arch, version)
	if goos == "darwin" {
		avalanchegoURL = fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-macos-%s.zip", version, version)
	}
	// EXPERMENTAL WIN, no support
	if goos == "windows" {
		avalanchegoURL = fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-win-%s-experimental.zip", version, version)
	}

	fmt.Printf("starting download from %s...\n", avalanchegoURL)

	resp, err = http.Get(avalanchegoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	archive, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println("download successful. installing archive...")
	if err := installArchive(goos, archive, binDir); err != nil {
		return "", err
	}
	return filepath.Join(binDir, "avalanchego-"+version), nil
}

func installArchive(goos string, archive []byte, binDir string) error {
	if goos == "darwin" || goos == "windows" {
		return installZipArchive(archive, binDir)
	}
	return installTarGzArchive(archive, binDir)
}

func installZipArchive(zipfile []byte, binDir string) error {
	bytesReader := bytes.NewReader(zipfile)
	zipReader, err := zip.NewReader(bytesReader, int64(len(zipfile)))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(binDir, f.Name)
		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(binDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range zipReader.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func installTarGzArchive(targz []byte, binDir string) error {
	byteReader := bytes.NewReader(targz)
	uncompressedStream, err := gzip.NewReader(byteReader)
	if err != nil {
		return fmt.Errorf("ExtractTarGz: NewReader failed: %s", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}
		// the target location where the dir/file should be created
		target := filepath.Join(binDir, header.Name)

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// copy over contents
			if _, err := io.Copy(f, tarReader); err != nil {
				return err
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func doDeploy(chain string) error {
	avagoDir, err := setupLocalEnv()
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("avalanchego installation successful")

	pluginDir := filepath.Join(avagoDir, "plugins")
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return fmt.Errorf("evaluated pluginDir to be %s but it does not exist.", pluginDir)
	}

	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return fmt.Errorf("evaluated avalancheGoBinPath to be %s but it does not exist.", avalancheGoBinPath)
	}

	requestTimeout := 3 * time.Minute

	cli, err := client.New(client.Config{
		LogLevel:    "info",
		Endpoint:    "0.0.0.0:8097",
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	chain_genesis := fmt.Sprintf("/home/fabio/.avalanche-cli/%s_genesis.json", chain)

	customVMs := map[string]string{
		chain: chain_genesis,
	}

	opts := []client.OpOption{
		// client.WithNumNodes(numNodes),
		client.WithPluginDir(pluginDir),
		client.WithCustomVMs(customVMs),
	}

	vmID, err := utils.VMID(chain)
	if err != nil {
		return err
	}
	fmt.Printf("this VM will get ID: %s\n", vmID.String())
	// if err := buildVM(chain, vmID, pluginDir); err != nil {
	if err := getVMBinary(vmID, pluginDir); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	fmt.Println("VM ready. Trying to boot network...")
	info, err := cli.Start(
		ctx,
		avalancheGoBinPath,
		opts...,
	)
	if err != nil {
		fmt.Printf("failed to start network :%s\n", err)
		return err
	}

	fmt.Println(info)
	fmt.Println("network has been booted. wait until healthy...")

	endpoints, err := waitForHealthy(ctx, cli)
	if err != nil {
		fmt.Printf("failed to query network health: %s\n", err)
		return err
	}

	fmt.Println("network ready to use. local network node endpoints:")
	for _, u := range endpoints {
		fmt.Println(u)
	}
	return nil
}

func printWait(cancel chan struct{}) {
	for {
		select {
		case <-time.After(1 * time.Second):
			fmt.Print(".")
		case <-cancel:
			return
		}
	}
}

func waitForHealthy(ctx context.Context, cli client.Client) ([]string, error) {
	cancel := make(chan struct{})
	go printWait(cancel)
	for {
		select {
		case <-ctx.Done():
			cancel <- struct{}{}
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			cancel <- struct{}{}
			fmt.Println()
			fmt.Println("polling for health...")
			resp, err := cli.Health(ctx)
			// TODO sometimes it hangs here!
			fmt.Println("health call returned")
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					return nil, err
				}
				fmt.Println("not yet")
				continue
			}
			if resp.ClusterInfo == nil {
				fmt.Println("warning: ClusterInfo is nil. trying again...")
				continue
			}
			if len(resp.ClusterInfo.CustomVms) == 0 {
				fmt.Println("network is up but custom VMs are not installed yet. polling again...")
				go printWait(cancel)
				time.Sleep(90 * time.Second)
				continue
			}
			endpoints := []string{}
			for _, nodeInfo := range resp.ClusterInfo.NodeInfos {
				for vmID, vmInfo := range resp.ClusterInfo.CustomVms {
					endpoints = append(endpoints, fmt.Sprintf("[blockchain RPC for %q] \"%s/ext/bc/%s\"{{/}}\n", vmID, nodeInfo.GetUri(), vmInfo.BlockchainId))
				}
			}
			fmt.Println("all good!")
			return endpoints, nil
		}
	}
}

func getVMBinary(id ids.ID, pluginDir string) error {
	vmID := id.String()
	binaryPath := filepath.Join(pluginDir, vmID)
	info, err := os.Stat(binaryPath)
	if err == nil {
		if !info.IsDir() {
			fmt.Println("binary already exists, skipping download")
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	fmt.Println("VM binary does not exist locally, starting download...")

	base, err := url.Parse(binaryServerURL)
	if err != nil {
		return err
	}

	// Path params
	// base.Path += "this will get automatically encoded"

	// Query params
	params := url.Values{}
	params.Add("vmid", vmID)
	base.RawQuery = params.Encode()

	fmt.Printf("starting download from %s...\n\n", base.String())

	resp, err := http.Get(base.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Println("download successful. installing binary...")
		return installBinary(bodyBytes, binaryPath)

	} else {
		return fmt.Errorf("downloading binary failed, status code: %d", resp.StatusCode)
	}
}

func installBinary(binary []byte, binaryPath string) error {
	if err := os.WriteFile(binaryPath, binary, 0o755); err != nil {
		return err
	}
	fmt.Println("binary installed. ready to go.")
	return nil
}

// this is NOT viable. Too many things can go wrong.
// i.e. cloning with git might require the ssh password...
func buildVM(chain string, id ids.ID, pluginDir string) error {
	fmt.Println("cloning subnet-evm...")
	subnetEVMRepo := "https://github.com/ava-labs/subnet-evm"
	// fatal: destination path '/home/fabio/go/src/github.com/ava-labs' already exists and is not an empty directory.
	// dest := "$HOME/go/src/github.com/ava-labs"
	dest := "/tmp/subnet-evm"
	args := []string{"clone", subnetEVMRepo, dest}
	// TODO git could not be installed...we need binaries
	clone := exec.Command("git", args...)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return err
	}

	fmt.Println("done. building...")
	// subnetEVMPath := "$HOME/go/src/github.com/ava-labs/subnet-evm/"
	buildPath := filepath.Join(dest, "scripts/build.sh")
	buildDest := filepath.Join("build", id.String())
	build := exec.Command(buildPath, buildDest)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	fmt.Println("done. copying to avalanchego plugin path...")
	binPath := filepath.Join(dest, buildDest)
	cpArgs := []string{binPath, pluginDir}
	cp := exec.Command("cp", cpArgs...)
	cp.Stdout = os.Stdout
	cp.Stderr = os.Stderr
	if err := cp.Run(); err != nil {
		return err
	}
	fmt.Println("all good")
	return nil
}

func startServerProcess() error {
	thisBin := reexec.Self()

	args := []string{"backend", "start"}
	cmd := exec.Command(thisBin, args...)
	outputFile, err := os.CreateTemp("", "avalanche-cli-backend*")
	if err != nil {
		return err
	}
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Start(); err != nil {
		return err
	}

	fmt.Printf("Backend controller started, pid: %d, output at: %s\n", cmd.Process.Pid, outputFile.Name())
	content := fmt.Sprintf("gRPC server output file: %s\ngRPC server PID: %d\n", outputFile.Name(), cmd.Process.Pid)
	err = os.WriteFile(serverRun, []byte(content), perms.ReadWrite)
	if err != nil {
		fmt.Printf("WARN: Could not write gRPC process info to file: %s\n", err)
	}
	return nil
}
