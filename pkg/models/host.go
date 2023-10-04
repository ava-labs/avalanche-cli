// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type Host struct {
	NodeID            string
	IP                string
	SSHUser           string
	SSHPrivateKeyPath string
	SSHCommonArgs     string
}
