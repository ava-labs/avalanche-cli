// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/subnet-evm/core"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/exp/slices"
	"golang.org/x/mod/semver"
)

func ErrWrongArgCount(expected, got int) error {
	return fmt.Errorf("requires %d arg(s), got %d", expected, got)
}

func SetupRealtimeCLIOutput(
	cmd *exec.Cmd,
	redirectStdout bool,
	redirectStderr bool,
) (*bytes.Buffer, *bytes.Buffer) {
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	if redirectStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuffer)
	} else {
		cmd.Stdout = io.MultiWriter(&stdoutBuffer)
	}
	if redirectStderr {
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)
	} else {
		cmd.Stderr = io.MultiWriter(&stderrBuffer)
	}
	return &stdoutBuffer, &stderrBuffer
}

// SplitKeyValueStringToMap splits a string with multiple key-value pairs separated by delimiter.
// Delimiter must be a single character
func SplitKeyValueStringToMap(str string, delimiter string) (map[string]string, error) {
	kvMap := make(map[string]string)
	if str == "" || len(delimiter) != 1 {
		return kvMap, nil
	}
	entries := SplitStringWithQuotes(str, rune(delimiter[0]))
	for _, e := range entries {
		parts := strings.Split(e, "=")
		if len(parts) >= 2 {
			kvMap[parts[0]] = strings.Trim(strings.Join(parts[1:], "="), "'")
		} else {
			kvMap[parts[0]] = strings.Trim(parts[0], "'")
		}
	}
	return kvMap, nil
}

// Context for ANR network operations
func GetANRContext() (context.Context, context.CancelFunc) {
	return GetTimedContext(constants.ANRRequestTimeout)
}

// Context for API requests
func GetAPIContext() (context.Context, context.CancelFunc) {
	return GetTimedContext(constants.APIRequestTimeout)
}

// Context for API requests with large timeout
func GetAPILargeContext() (context.Context, context.CancelFunc) {
	return GetTimedContext(constants.APIRequestLargeTimeout)
}

// Timed Context
func GetTimedContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func GetRealFilePath(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = strings.Replace(path, "~", usr.HomeDir, 1)
	}
	return path
}

func Any[T any](input []T, f func(T) bool) bool {
	for _, e := range input {
		if f(e) {
			return true
		}
	}
	return false
}

func Find[T any](input []T, f func(T) bool) *T {
	for _, e := range input {
		if f(e) {
			return &e
		}
	}
	return nil
}

func RemoveFromSlice[T comparable](input []T, toRemove T) []T {
	output := make([]T, 0, len(input))
	for _, e := range input {
		if e != toRemove {
			output = append(output, e)
		}
	}
	return output
}

func Filter[T any](input []T, f func(T) bool) []T {
	output := make([]T, 0, len(input))
	for _, e := range input {
		if f(e) {
			output = append(output, e)
		}
	}
	return output
}

func MapWithError[T, U any](input []T, f func(T) (U, error)) ([]U, error) {
	output := make([]U, 0, len(input))
	for _, e := range input {
		o, err := f(e)
		if err != nil {
			return nil, err
		}
		output = append(output, o)
	}
	return output, nil
}

// ConvertInterfaceToMap converts a given value to a map[string]interface{}.
func ConvertInterfaceToMap(value interface{}) (map[string]interface{}, error) {
	// Check if the underlying type is a map
	switch v := value.(type) {
	case map[string]interface{}:
		// If it's a map, return it
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", value)
	}
}

// IsUnsignedSlice returns true if all elements in the slice are unsigned integers.
func IsUnsignedSlice(n []int) bool {
	for _, v := range n {
		if v < 0 {
			return false
		}
	}
	return true
}

// TimedFunction is a function that executes the given function `f` within a specified timeout duration.
func TimedFunction[T any](
	f func() (T, error),
	name string,
	timeout time.Duration,
) (T, error) {
	var (
		ret T
		err error
	)
	ch := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		ret, err = f()
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return ret, fmt.Errorf("%s timeout of %d seconds", name, uint(timeout.Seconds()))
	case <-ch:
	}
	return ret, err
}

// TimedFunctionWithRetry is a function that executes the given function `f` within a specified timeout duration.
func TimedFunctionWithRetry[T any](
	f func() (T, error),
	name string,
	timeout time.Duration,
	maxAttempts int,
	retryInterval time.Duration,
) (T, error) {
	return utils.Retry(
		func() (T, error) {
			return TimedFunction(f, name, timeout)
		},
		maxAttempts,
		retryInterval,
	)
}

// Unique returns a new slice containing only the unique elements from the input slice.
func Unique(slice []string) []string {
	visited := make(map[string]bool)
	uniqueSlice := make([]string, 0)
	for _, element := range slice {
		if !visited[element] {
			// If the element is not visited, add it to the uniqueSlice
			uniqueSlice = append(uniqueSlice, element)
			visited[element] = true
		}
	}
	return uniqueSlice
}

// ContainsIgnoreCase checks if the given string contains the specified substring, ignoring case.
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// SplitSliceAt splits a slice at the given index and returns two new slices.func SplitSliceAt[T any](slice []T, index int) ([]T, []T) {
func SplitSliceAt[T any](slice []T, index int) ([]T, []T) {
	if index < 0 || index >= len(slice) {
		return slice, nil
	}
	if index == 0 {
		return nil, slice
	}
	return slice[:index], slice[index:]
}

// RandomString generates a random string of the specified length.
func RandomString(length int) string {
	randG := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	chars := "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[randG.Intn(len(chars))]
	}
	return string(result)
}

// Sum calculates the sum of all the elements in the given slice of integers.
func Sum(s []int) int {
	sum := 0
	for _, v := range s {
		sum += v
	}
	return sum
}

func ScriptLog(nodeID string, msg string, args ...interface{}) string {
	formattedMsg := fmt.Sprintf(msg, args...)
	return fmt.Sprintf("[%s] %s", nodeID, formattedMsg)
}

func GetIndexInSlice[T comparable](list []T, element T) (int, error) {
	for i, val := range list {
		if val == element {
			return i, nil
		}
	}
	return 0, fmt.Errorf("element not found")
}

// GetRepoFromCommitURL takes a Git repository URL that contains commit ID and returns the cloneable
// Git Repo URL (ends in .git) and the repo directory name
// Example: https://github.com/ava-labs/hypersdk/pull/772/commits/b88acfb370f5aeb83a000aece2d72f28154410a5
// Should return https://github.com/ava-labs/hypersdk
func GetRepoFromCommitURL(gitRepoURL string) (string, string) {
	splitURL := strings.Split(gitRepoURL, "/")
	if len(splitURL) > 4 {
		// get first five members of splitURL because it will be [ https, ' ', github.com, ava-labs, hypersdk]
		splitURLWOCommit := splitURL[:5]
		gitRepo := strings.Join(splitURLWOCommit, "/")
		return gitRepo, splitURLWOCommit[4]
	}
	return "", ""
}

// GetGitCommit takes a Git repository URL that contains commit ID and returns the commit ID
// Example: https://github.com/ava-labs/hypersdk/pull/772/commits/b88acfb370f5aeb83a000aece2d72f28154410a5
// Should return b88acfb370f5aeb83a000aece2d72f28154410a5
func GetGitCommit(gitRepoURL string) string {
	if strings.Contains(gitRepoURL, "/commit") {
		splitURL := strings.Split(gitRepoURL, "/")
		if len(splitURL) > 0 {
			commitID := splitURL[len(splitURL)-1]
			return commitID
		}
	}
	return ""
}

// ReadLongString reads a long string from the user input.
func ReadLongString(msg string, args ...interface{}) (string, error) {
	fmt.Printf(msg, args...)
	reader := bufio.NewReader(os.Stdin)
	longString, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// Remove newline character at the end
	longString = strings.TrimSuffix(longString, "\n")
	return longString, nil
}

func SupportedAvagoArch() []string {
	return []string{string(types.ArchitectureTypeArm64), string(types.ArchitectureTypeX8664)}
}

func ArchSupported(arch string) bool {
	return slices.Contains(SupportedAvagoArch(), arch)
}

// Get the host, port and path from a URL.
func GetURIHostPortAndPath(uri string) (string, uint32, string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to parse uri %s: %w", uri, err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to split host/port at uri %s: %w", uri, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to convert port to uint at uri %s: %w", uri, err)
	}
	return host, uint32(port), u.Path, nil
}

func GetCodespaceURL(url string) (string, error) {
	codespaceName := os.Getenv(constants.CodespaceNameEnvVar)
	if codespaceName == "" {
		return "", nil
	}
	if strings.HasPrefix(url, constants.MainnetAPIEndpoint) || strings.HasPrefix(url, constants.FujiAPIEndpoint) {
		return "", nil
	}
	_, port, path, err := GetURIHostPortAndPath(url)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s-%d.app.github.dev%s", codespaceName, port, path), nil
}

func InsideCodespace() bool {
	return os.Getenv(constants.CodespaceNameEnvVar) != ""
}

func GetChainID(endpoint string, chainName string) (ids.ID, error) {
	client := info.NewClient(endpoint)
	ctx, cancel := GetAPIContext()
	defer cancel()
	return client.GetBlockchainID(ctx, chainName)
}

func GetChainIDs(endpoint string, chainName string) (string, string, error) {
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := GetAPIContext()
	defer cancel()
	blockChains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return "", "", err
	}
	if chain := Find(blockChains, func(e platformvm.APIBlockchain) bool { return e.Name == chainName }); chain != nil {
		return chain.SubnetID.String(), chain.ID.String(), nil
	}
	return "", "", fmt.Errorf("%s not found on primary network blockchains", chainName)
}

func GetNodeID(endpoint string) (
	string, // nodeID
	string, // public key
	string, // PoP
	error,
) {
	infoClient := info.NewClient(endpoint)
	ctx, cancel := GetAPILargeContext()
	defer cancel()
	nodeID, proofOfPossession, err := infoClient.GetNodeID(ctx)
	if err != nil {
		return "", "", "", err
	}
	return nodeID.String(),
		"0x" + hex.EncodeToString(proofOfPossession.PublicKey[:]),
		"0x" + hex.EncodeToString(proofOfPossession.ProofOfPossession[:]),
		nil
}

func GetBlockchainTx(endpoint string, blockchainID ids.ID) (*txs.CreateChainTx, error) {
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := GetAPIContext()
	defer cancel()
	txBytes, err := pClient.GetTx(ctx, blockchainID)
	if err != nil {
		return nil, err
	}
	var tx txs.Tx
	if _, err = txs.Codec.Unmarshal(txBytes, &tx); err != nil {
		return nil, fmt.Errorf("failed unmarshaling the createChainTx: %w", err)
	}
	createChainTx, ok := tx.Unsigned.(*txs.CreateChainTx)
	if !ok {
		return nil, fmt.Errorf("expected a CreateChainTx, got %T", tx.Unsigned)
	}
	return createChainTx, nil
}

func ByteSliceToSubnetEvmGenesis(bs []byte) (core.Genesis, error) {
	var gen core.Genesis
	err := json.Unmarshal(bs, &gen)
	return gen, err
}

func ByteSliceIsSubnetEvmGenesis(bs []byte) bool {
	_, err := ByteSliceToSubnetEvmGenesis(bs)
	return err == nil
}

func FileIsSubnetEVMGenesis(genesisPath string) (bool, error) {
	genesisBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		return false, err
	}
	return ByteSliceIsSubnetEvmGenesis(genesisBytes), nil
}

func GetDefaultBlockchainAirdropKeyName(blockchainName string) string {
	return "subnet_" + blockchainName + "_airdrop"
}

// AppendSlices appends multiple slices into a single slice.
func AppendSlices[T any](slices ...[]T) []T {
	totalLength := 0
	for _, slice := range slices {
		totalLength += len(slice)
	}
	result := make([]T, 0, totalLength)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

// ExtractValueFromBytes extracts a value from a byte array using a regular expression.
func ExtractPlaceholderValue(pattern, text string) (string, error) {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) == 2 {
		return matches[1], nil
	} else {
		return "", fmt.Errorf("no match found")
	}
}

// Command returns an exec.Cmd for the given command line.
func Command(cmdLine string, params ...string) *exec.Cmd {
	cmd := strings.Split(cmdLine, " ")
	cmd = append(cmd, params...)
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = os.Environ()
	return c
}

// StringValue returns the value of a key in a map as a string.
func StringValue(data map[string]interface{}, key string) (string, error) {
	if value, ok := data[key]; ok {
		return fmt.Sprintf("%v", value), nil
	}
	return "", fmt.Errorf("key %s not found", key)
}

func LogLevelToEmoji(logLevel string) (string, error) {
	levelEmoji := ""
	level, err := logging.ToLevel(logLevel)
	if err != nil {
		return "", err
	}
	switch level {
	case logging.Info:
		levelEmoji = "ℹ️"
	case logging.Debug:
		levelEmoji = "🪲"
	case logging.Warn:
		levelEmoji = "⚠️"
	case logging.Error:
		levelEmoji = "⛔"
	case logging.Fatal:
		levelEmoji = "💀"
	}
	return levelEmoji, nil
}

func IsValidSemanticVersion(version string, component string) bool {
	if !semver.IsValid(version) {
		versionTail := strings.TrimPrefix(version, component+"-")
		if semver.IsValid(versionTail) {
			return true
		}
		versionTail = strings.TrimPrefix(version, component+"/")
		return semver.IsValid(versionTail)
	}
	return true
}

// PrintUnreportedErrors takes a list of errors obtained by a routine, and the main error
// it is going to report, and prints to the user the unreported errors only,
// avoiding duplications
func PrintUnreportedErrors(
	errors []error,
	returnedError error,
	print func(string, ...interface{}),
) {
	if returnedError == nil {
		return
	}
	errSet := set.Set[string]{}
	errSet.Add(returnedError.Error())
	for _, err := range errors {
		if !errSet.Contains(err.Error()) {
			print(err.Error())
			errSet.Add(err.Error())
		}
	}
}

func NewLogger(
	logName string,
	logLevelStr string,
	defaultLogLevelStr string,
	logDir string,
	logToStdout bool,
	print func(string, ...interface{}),
) (logging.Logger, error) {
	logLevel, err := logging.ToLevel(logLevelStr)
	if err != nil {
		if logLevelStr != "" {
			print("undefined logLevel %s. Setting %s log to %s", logLevelStr, logName, defaultLogLevelStr)
		}
		logLevel, err = logging.ToLevel(defaultLogLevelStr)
		if err != nil {
			return logging.NoLog{}, err
		}
	}
	logConfig := logging.Config{
		RotatingWriterConfig: logging.RotatingWriterConfig{
			Directory: logDir,
		},
		LogLevel: logLevel,
	}
	if logToStdout {
		logConfig.DisplayLevel = logLevel
	}
	logFactory := logging.NewFactory(logConfig)
	return logFactory.Make(logName)
}

func GetProcess(pid int) (*os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// sometimes FindProcess returns without error, but Signal 0 will surely fail if the process doesn't exist
		return nil, err
	}
	return proc, nil
}

func VMID(vmName string) (ids.ID, error) {
	if len(vmName) > 32 {
		return ids.Empty, fmt.Errorf("VM name must be <= 32 bytes, found %d", len(vmName))
	}
	b := make([]byte, 32)
	copy(b, []byte(vmName))
	return ids.ToID(b)
}

func MkDirWithTimestamp(dirPrefix string) (string, error) {
	const dirTimestampFormat = "20060102_150405"
	currentTime := time.Now().Format(dirTimestampFormat)
	dirName := dirPrefix + "_" + currentTime
	return dirName, os.MkdirAll(dirName, os.ModePerm)
}

func PointersSlice[T any](input []T) []*T {
	output := make([]*T, 0, len(input))
	for _, e := range input {
		output = append(output, &e)
	}
	return output
}
