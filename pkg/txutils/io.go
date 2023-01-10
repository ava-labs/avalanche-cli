// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package txutils

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

// saves a given [tx] to [txPath]
func SaveToDisk(tx *txs.Tx, txPath string, forceOverwrite bool) error {
	// Serialize the signed tx
	txBytes, err := txs.Codec.Marshal(txs.Version, tx)
	if err != nil {
		return fmt.Errorf("couldn't marshal signed tx: %w", err)
	}

	// Get the encoded (in hex + checksum) signed tx
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode signed tx: %w", err)
	}
	// save
	if _, err := os.Stat(txPath); err == nil && !forceOverwrite {
		return fmt.Errorf("couldn't create file to write tx to: file exists")
	}
	f, err := os.Create(txPath)
	if err != nil {
		return fmt.Errorf("couldn't create file to write tx to: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(txStr)
	if err != nil {
		return fmt.Errorf("couldn't write tx into file: %w", err)
	}
	return nil
}

// loads a tx from [txPath]
func LoadFromDisk(txPath string) (*txs.Tx, error) {
	txEncodedBytes, err := os.ReadFile(txPath)
	if err != nil {
		return nil, err
	}
	txBytes, err := formatting.Decode(formatting.Hex, string(txEncodedBytes))
	if err != nil {
		return nil, fmt.Errorf("couldn't decode signed tx: %w", err)
	}
	var tx txs.Tx
	if _, err := txs.Codec.Unmarshal(txBytes, &tx); err != nil {
		return nil, fmt.Errorf("error unmarshaling signed tx: %w", err)
	}
	if err := tx.Initialize(txs.Codec); err != nil {
		return nil, fmt.Errorf("error initializing signed tx: %w", err)
	}
	return &tx, nil
}
