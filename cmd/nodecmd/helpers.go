// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

// NumNodes is a struct to hold number of nodes with and without stake
type NumNodes struct {
	numValidators int // with stake
	numAPI        int // without stake
}

func (n NumNodes) All() int {
	return n.numValidators + n.numAPI
}
