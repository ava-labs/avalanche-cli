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

type HostFuncWithParams struct {
	Func   func(host Host, params interface{}) error
	Params interface{}
}

func (f *HostFuncWithParams) Run(host Host) error {
	return f.Func(host, f.Params)
}
