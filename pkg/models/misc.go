// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type NodeStringResult struct {
	NodeID string
	Value  string
	Err    error
}

type NodeErrorResult struct {
	NodeID string
	Err    error
}

type NodeBooleanResult struct {
	NodeID string
	Value  bool
	Err    error
}
