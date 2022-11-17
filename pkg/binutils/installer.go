// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"runtime"
)

type Installer interface {
	GetArch() (string, string)
}

type installerImpl struct{}

func NewInstaller() Installer {
	return &installerImpl{}
}

func (installerImpl) GetArch() (string, string) {
	return runtime.GOARCH, runtime.GOOS
}
