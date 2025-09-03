// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cubist

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/wallet/chain/p"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	pbuilder "github.com/ava-labs/avalanchego/wallet/chain/p/builder"
	pwallet "github.com/ava-labs/avalanchego/wallet/chain/p/wallet"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/cubist-labs/cubesigner-go-sdk/client"
	"github.com/cubist-labs/cubesigner-go-sdk/models"
	"github.com/cubist-labs/cubesigner-go-sdk/session"
	"github.com/spf13/cobra"
)

func callDemo(_ *cobra.Command, _ []string) error {
	filePath := "/Users/raymondsukanto/Desktop/management-session.json"
	manager, err := session.NewJsonSessionManager(&filePath)
	if err != nil {
		return err
	}
	apiClient, err := client.NewApiClient(manager)
	if err != nil {
		return err
	}

	sampleAddr := "P-fuji1u8933yvsmf5d6cqkm3qgewzlpr7sac3v3eufj9"
	destinationAddr, err := address.ParseToID(sampleAddr)
	if err != nil {
		return err
	}
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(destinationAddr)

	newPWallet, _, builder, err := CreateReadOnlyWallet("https://api.avax-test.network", customAddrsSet, primary.WalletConfig{})
	if err != nil {
		return err
	}

	owners := &secp256k1fx.OutputOwners{
		Addrs:     []ids.ShortID{destinationAddr},
		Threshold: 1,
		Locktime:  0,
	}
	tx, err := builder.NewCreateSubnetTx(owners)
	if err != nil {
		return err
	}

	// Serialize the signed tx
	txBytes, err := txs.Codec.Marshal(txs.CodecVersion, tx)
	if err != nil {
		return fmt.Errorf("couldn't marshal signed tx: %w", err)
	}

	// Get the encoded (in hex + checksum) signed tx
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode signed tx: %w", err)
	}
	fmt.Printf("txStr %s \n", txStr)
	avaRequest := models.AvaSerializedTxSignRequest{
		Tx: txStr,
	}
	response, err := apiClient.AvaSerializedTxSign("P", "fuji1u8933yvsmf5d6cqkm3qgewzlpr7sac3v3eufj9", avaRequest)
	if err != nil {
		return fmt.Errorf("response err: %w", err)
	}

	// Extract the actual signed transaction string and use it
	if response.ResponseData != nil {
		b, err := json.MarshalIndent(response.ResponseData, "", " ")
		if err != nil {
			return fmt.Errorf("MarshalIndent err: %w", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("unmarshal err: %w", err)
		}
		signedTxStr, _ := m["signature"].(string)
		fmt.Printf("signature %s", signedTxStr)

		if signedTxStr[:2] == "0x" {
			signedTxStr = signedTxStr[2:]
		}
		fmt.Printf("signedTxStr %s \n", signedTxStr)
		// Decode hex to bytes
		txBytes, err := hex.DecodeString(signedTxStr)
		if err != nil {
			log.Fatalf("Failed to decode hex: %v", err)
		}

		cred := &secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		}
		copy(cred.Sigs[0][:], txBytes)

		signedTx := &txs.Tx{
			Unsigned: tx,
			Creds:    []verify.Verifiable{cred},
		}
		// Initialize the transaction (this sets the transaction ID)
		if err := signedTx.Initialize(txs.Codec); err != nil {
			log.Fatalf("Failed to initialize transaction: %v", err)
		}
		fmt.Printf("Transaction ID: %s\n", signedTx.ID())

		txID, err := issueTx(*newPWallet, signedTx)
		if err != nil {
			return fmt.Errorf("failed to issue tx: %w", err)
		}
		fmt.Printf("issued txid %s \n", txID.String())
	}

	return nil
}

func issueTx(newPWallet pwallet.Wallet, tx *txs.Tx) (ids.ID, error) {
	const (
		repeats             = 3
		sleepBetweenRepeats = 2 * time.Second
	)
	var (
		issueTxErr error
		errors     []error
	)
	for i := 0; i < repeats; i++ {
		ctx, cancel := sdkutils.GetAPILargeContext()
		defer cancel()
		options := []common.Option{common.WithContext(ctx)}
		issueTxErr = newPWallet.IssueTx(tx, options...)
		if issueTxErr == nil {
			break
		}
		if ctx.Err() != nil {
			issueTxErr = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), issueTxErr)
		} else {
			issueTxErr = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), issueTxErr)
		}
		errors = append(errors, issueTxErr)
		time.Sleep(sleepBetweenRepeats)
	}
	utils.PrintUnreportedErrors(errors, issueTxErr, ux.Logger.PrintToUser)
	return tx.ID(), issueTxErr
}

func CreateReadOnlyWallet(
	uri string,
	addresses set.Set[ids.ShortID],
	config primary.WalletConfig,
) (*pwallet.Wallet, *p.Client, pbuilder.Builder, error) {

	ctx := context.Background()

	avaxState, err := primary.FetchState(ctx, uri, addresses)
	if err != nil {
		return nil, nil, nil, err
	}

	owners, err := platformvm.GetOwners(avaxState.PClient, ctx, config.SubnetIDs, config.ValidationIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	pUTXOs := common.NewChainUTXOs(constants.PlatformChainID, avaxState.UTXOs)
	pBackend := pwallet.NewBackend(avaxState.PCTX, pUTXOs, owners)
	pClient := p.NewClient(avaxState.PClient, pBackend)
	pBuilder := pbuilder.New(addresses, avaxState.PCTX, pBackend)
	newPWallet := pwallet.New(pClient, pBuilder, nil)

	return &newPWallet, pClient, pBuilder, nil
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a signing key",
		Long: `The key create command generates a new private key to use for creating and controlling
test Subnets. Keys generated by this command are NOT cryptographically secure enough to
use in production environments. DO NOT use these keys on Mainnet.

The command works by generating a secp256 key and storing it with the provided keyName. You
can use this key in other commands by providing this keyName.

If you'd like to import an existing key instead of generating one from scratch, provide the
--file flag.`,
		Args: cobrautils.ExactArgs(0),
		RunE: callDemo,
	}

	return cmd
}
