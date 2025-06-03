// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureaggregator

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func NewSignatureAggregatorLogger(
	aggregatorLogLevel string,
	aggregatorLogToStdout bool,
	logDir string,
) (logging.Logger, error) {
	return utils.NewLogger(
		constants.SignatureAggregator,
		aggregatorLogLevel,
		constants.DefaultAggregatorLogLevel,
		logDir,
		aggregatorLogToStdout,
		ux.Logger.PrintToUser,
	)
}

func GetLatestSignatureAggregatorReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.SignatureAggregator,
	)
}

func GetLatestSignatureAggregatorPreReleaseVersion() (string, error) {
	downloader := application.NewDownloader()
	return downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		constants.SignatureAggregator,
	)
}

func InstallSignatureAggregator(binDir, version string) (string, error) {
	if version == "" || version == constants.LatestPreReleaseVersionTag {
		var err error
		version, err = GetLatestSignatureAggregatorPreReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	if version == constants.LatestReleaseVersionTag {
		var err error
		version, err = GetLatestSignatureAggregatorReleaseVersion()
		if err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Signature Aggregator version %s", version)
	versionBinDir := filepath.Join(binDir, version)
	binPath := filepath.Join(versionBinDir, constants.SignatureAggregator)
	if utils.IsExecutable(binPath) {
		return binPath, nil
	}
	ux.Logger.PrintToUser("Installing Signature Aggregator")
	url, err := getSignatureAggregatorURL(version)
	if err != nil {
		return "", err
	}
	bs, err := utils.Download(url)
	if err != nil {
		return "", err
	}
	if err := binutils.InstallArchive("tar.gz", bs, versionBinDir); err != nil {
		return "", err
	}
	return binPath, nil
}

func getSignatureAggregatorURL(version string) (string, error) {
	goarch, goos := runtime.GOARCH, runtime.GOOS
	if goos != "linux" && goos != "darwin" {
		return "", fmt.Errorf("OS not supported: %s", goos)
	}
	component := "signature-aggregator"
	semanticVersion := strings.TrimPrefix(version, component+"/")
	if semanticVersion != version {
		return fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/signature-aggregator%%2F%s/signature-aggregator_%s_%s_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.ICMServicesRepoName,
			semanticVersion,
			strings.TrimPrefix(semanticVersion, "v"),
			goos,
			goarch,
		), nil
	}
	semanticVersion = strings.TrimPrefix(version, component+"-")
	if semanticVersion != version {
		return fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/signature-aggregator-%s/signature-aggregator_%s_%s_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.ICMServicesRepoName,
			semanticVersion,
			strings.TrimPrefix(semanticVersion, "v"),
			goos,
			goarch,
		), nil
	}
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/signature-aggregator_%s_%s_%s.tar.gz",
		constants.AvaLabsOrg,
		constants.ICMServicesRepoName,
		semanticVersion,
		strings.TrimPrefix(semanticVersion, "v"),
		goos,
		goarch,
	), nil
}
