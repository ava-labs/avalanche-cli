// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestContractAddressIsInGenesisData(t *testing.T) {
	require := require.New(t)

	type test struct {
		desc            string
		genesisData     []byte
		contractAddress common.Address
		expected        bool
		shouldErr       bool
	}

	tests := []test{
		{
			desc:            "nil data",
			genesisData:     nil,
			contractAddress: common.Address{},
			expected:        false,
			shouldErr:       true,
		},
		{
			desc:            "not json",
			genesisData:     []byte("not json"),
			contractAddress: common.Address{},
			expected:        false,
			shouldErr:       true,
		},
		{
			desc:            "not evm",
			genesisData:     []byte("{}"),
			contractAddress: common.Address{},
			expected:        false,
			shouldErr:       true,
		},
		{
			desc: "no allocs",
			genesisData: []byte(`
{
    "config": {
        "byzantiumBlock": 0, "chainId": 1, "constantinopleBlock": 0, "eip150Block": 0,
        "eip155Block": 0, "eip158Block": 0,
        "feeConfig": {
            "gasLimit": 12000000, "targetBlockRate": 2, "minBaseFee": 25000000000,
            "targetGas": 60000000, "baseFeeChangeDenominator": 36, "minBlockGasCost": 0,
            "maxBlockGasCost": 1000000, "blockGasCostStep": 200000
        },
        "homesteadBlock": 0, "istanbulBlock": 0, "muirGlacierBlock": 0, "petersburgBlock": 0,
        "warpConfig": {
            "blockTimestamp": 1727309619, "quorumNumerator": 67
        }
    },
    "nonce": "0x0", "timestamp": "0x66f4a733", "extraData": "0x", "gasLimit": "0xb71b00", "difficulty": "0x0",
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "airdropHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "airdropAmount": null, "number": "0x0", "gasUsed": "0x0",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "baseFeePerGas": null, "excessBlobGas": null, "blobGasUsed": null,
    "alloc": {}
}
            `),
			contractAddress: common.Address{},
			expected:        false,
			shouldErr:       false,
		},
		{
			desc: "good path",
			genesisData: []byte(`
{
    "config": {
        "byzantiumBlock": 0, "chainId": 1, "constantinopleBlock": 0, "eip150Block": 0,
        "eip155Block": 0, "eip158Block": 0,
        "feeConfig": {
            "gasLimit": 12000000, "targetBlockRate": 2, "minBaseFee": 25000000000,
            "targetGas": 60000000, "baseFeeChangeDenominator": 36, "minBlockGasCost": 0,
            "maxBlockGasCost": 1000000, "blockGasCostStep": 200000
        },
        "homesteadBlock": 0, "istanbulBlock": 0, "muirGlacierBlock": 0, "petersburgBlock": 0,
        "warpConfig": {
            "blockTimestamp": 1727309619, "quorumNumerator": 67
        }
    },
    "nonce": "0x0", "timestamp": "0x66f4a733", "extraData": "0x", "gasLimit": "0xb71b00", "difficulty": "0x0",
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "airdropHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "airdropAmount": null, "number": "0x0", "gasUsed": "0x0",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "baseFeePerGas": null, "excessBlobGas": null, "blobGasUsed": null,
    "alloc": {
        "253b2784c75e510dd0ff1da844684a1ac0aa5fcf": {
            "code": "0xfe",
            "balance": "0x2086ac351052600000"
        }
    }
}
            `),
			contractAddress: common.HexToAddress("0x253b2784c75e510dd0ff1da844684a1ac0aa5fcf"),
			expected:        true,
			shouldErr:       false,
		},
		{
			desc: "no code",
			genesisData: []byte(`
{
    "config": {
        "byzantiumBlock": 0, "chainId": 1, "constantinopleBlock": 0, "eip150Block": 0,
        "eip155Block": 0, "eip158Block": 0,
        "feeConfig": {
            "gasLimit": 12000000, "targetBlockRate": 2, "minBaseFee": 25000000000,
            "targetGas": 60000000, "baseFeeChangeDenominator": 36, "minBlockGasCost": 0,
            "maxBlockGasCost": 1000000, "blockGasCostStep": 200000
        },
        "homesteadBlock": 0, "istanbulBlock": 0, "muirGlacierBlock": 0, "petersburgBlock": 0,
        "warpConfig": {
            "blockTimestamp": 1727309619, "quorumNumerator": 67
        }
    },
    "nonce": "0x0", "timestamp": "0x66f4a733", "extraData": "0x", "gasLimit": "0xb71b00", "difficulty": "0x0",
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "airdropHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "airdropAmount": null, "number": "0x0", "gasUsed": "0x0",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "baseFeePerGas": null, "excessBlobGas": null, "blobGasUsed": null,
    "alloc": {
        "253b2784c75e510dd0ff1da844684a1ac0aa5fcf": {
            "balance": "0x2086ac351052600000"
        }
    }
}
            `),
			contractAddress: common.HexToAddress("0x253b2784c75e510dd0ff1da844684a1ac0aa5fcf"),
			expected:        false,
			shouldErr:       false,
		},
		{
			desc: "diff addr",
			genesisData: []byte(`
{
    "config": {
        "byzantiumBlock": 0, "chainId": 1, "constantinopleBlock": 0, "eip150Block": 0,
        "eip155Block": 0, "eip158Block": 0,
        "feeConfig": {
            "gasLimit": 12000000, "targetBlockRate": 2, "minBaseFee": 25000000000,
            "targetGas": 60000000, "baseFeeChangeDenominator": 36, "minBlockGasCost": 0,
            "maxBlockGasCost": 1000000, "blockGasCostStep": 200000
        },
        "homesteadBlock": 0, "istanbulBlock": 0, "muirGlacierBlock": 0, "petersburgBlock": 0,
        "warpConfig": {
            "blockTimestamp": 1727309619, "quorumNumerator": 67
        }
    },
    "nonce": "0x0", "timestamp": "0x66f4a733", "extraData": "0x", "gasLimit": "0xb71b00", "difficulty": "0x0",
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "coinbase": "0x0000000000000000000000000000000000000000",
    "airdropHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "airdropAmount": null, "number": "0x0", "gasUsed": "0x0",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "baseFeePerGas": null, "excessBlobGas": null, "blobGasUsed": null,
    "alloc": {
        "253b2784c75e510dd0ff1da844684a1ac0aa5fcf": {
            "code": "0xfe",
            "balance": "0x2086ac351052600000"
        }
    }
}
            `),
			contractAddress: common.HexToAddress("0x253b2724c75e510dd0ff1da844684a1ac0aa5fcc"),
			expected:        false,
			shouldErr:       false,
		},
	}

	for _, t := range tests {
		b, err := ContractAddressIsInGenesisData(t.genesisData, t.contractAddress)
		if t.shouldErr {
			require.Error(err, t.desc)
		} else {
			require.NoError(err, t.desc)
		}
		require.Equal(t.expected, b, t.desc)
	}
}
