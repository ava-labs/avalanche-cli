// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cubist

import (
	"context"
	"encoding/json"
	"fmt"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"time"

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
	"github.com/cubist-labs/cubesigner-go-sdk/utils/ref"
	"github.com/spf13/cobra"
)

const (
	forceFlag = "force"
)

var (
	forceCreate  bool
	skipBalances bool
	filename     string
)

func callDemo(_ *cobra.Command, args []string) error {
	//manager, err := session.NewJsonSessionManager(nil)
	//if err != nil {
	//	return err
	//}
	filePath := "/Users/raymondsukanto/Desktop/management-session.json"
	manager, err := session.NewJsonSessionManager(&filePath)
	if err != nil {
		fmt.Printf("we have err here %s", err)
		return err
	}
	fmt.Printf("manager path %s \n", manager.FilePath)
	apiClient, err := client.NewApiClient(manager)
	if err != nil {
		fmt.Printf("we have err here 2 %s", err)
		return err
	}

	//// Get user info
	//userInfo, err := apiClient.AboutMe()
	//if err != nil {
	//	fmt.Printf("we have err here 3 %s", err)
	//	return err
	//}
	//
	//// each user has a globally unique ID
	//userId := userInfo.UserId
	//if err != nil {
	//	return err
	//}
	//fmt.Printf("userId %s \n", userId)
	//
	//// Ids of all organizations this user is a member of
	//allOrgs := userInfo.Orgs
	//if err != nil {
	//	return err
	//}
	//fmt.Printf("allOrgs %s \n", allOrgs)
	//f

	//createKeyRequest := models.CreateKeyRequest{KeyType: models.SecpAvaTestAddr, Count: 1}
	//keysInfo, err := apiClient.CreateKey(createKeyRequest)
	//if err != nil {
	//	return err
	//}

	//// read response to get the key info
	//secpKey := keysInfo.Keys[0]
	//fmt.Printf("secpKey %s \n", secpKey.KeyId)
	//fmt.Printf("secpKey %s \n", secpKey.MaterialId)

	TxBody := models.TypedTransaction{}
	TxBody.FromTypedTransactionEip1559(models.TypedTransactionEip1559{
		To:                   ref.Of("0xff50ed3d0ec03ac01d4c79aad74928bff48a7b2b"),
		Type:                 "0x02",
		Gas:                  ref.Of("0x61a80"),
		MaxFeePerGas:         ref.Of("0x2540be400"),
		MaxPriorityFeePerGas: ref.Of("0x3b9aca00"),
		Nonce:                ref.Of("0"),
		Value:                ref.Of("0x100"),
	})

	//eth1Request := models.Eth1SignRequest{
	//	ChainId: int64(1),
	//	Tx:      TxBody,
	//}
	sampleAddr := "P-fuji1u8933yvsmf5d6cqkm3qgewzlpr7sac3v3eufj9"
	destinationAddr, err := address.ParseToID(sampleAddr)
	if err != nil {
		return err
	}
	customAddrsSet := set.Set[ids.ShortID]{}
	customAddrsSet.Add(destinationAddr)
	//avaxState, err := FetchState(ctx, uri, avaxAddrs)
	//if err != nil {
	//	return nil, err
	//}
	//pClient := p.NewClient(avaxState.PClient, customAddrsSet)
	builder, err := CreateReadOnlyWallet("https://api.avax-test.network", customAddrsSet, primary.WalletConfig{})
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

	//// Create the transaction structure manually
	//tx := &txs.CreateSubnetTx{
	//	BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
	//		NetworkID: networkID,
	//		Ins:       inputs, // You'd need to build these
	//	}},
	//	Owner: owners,
	//}
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
	if response.ResponseData != nil {
		fmt.Printf("no err response %s \n", *response.ResponseData)
	} else {
		fmt.Printf("no err response: ResponseData is nil\n")
	}
	fmt.Printf("no err response %s \n", response.MfaRequired)
	b, err := json.MarshalIndent(response.ResponseData, "", "  ")
	if err != nil {
		return fmt.Errorf("MarshalIndent err: %w", err)
	} else {
		fmt.Printf("obtained string %s \n", string(b))
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("unmarshal err: %w", err)
	}
	sig, _ := m["signature"].(string)
	fmt.Println("signature:", sig)

	// Extract the actual signed transaction string and use it
	if response.ResponseData != nil {
		fmt.Printf("Signed transaction: %s\n", *response.ResponseData)

		// Parse the signed transaction
		signedTx, err := getSignedTx(sig)
		if err != nil {
			return fmt.Errorf("failed to parse signed transaction: %w", err)
		}

		// Now you can use the signedTx
		fmt.Printf("Successfully parsed signed transaction with ID: %s\n", signedTx.ID())

		// TODO: Use the signed transaction (e.g., submit to network)
		// pWallet := pwallet.New(pClient, builder, nil)
		// pWallet.IssueCreateSubnetTx()
	} else {
		return fmt.Errorf("ResponseData is nil - no signed transaction received")
	}

	return nil
}

func issueTx(tx txs.Tx) (ids.ID, error) {
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
		issueTxErr = wallet.P().IssueTx(tx, options...)
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
	return tx.ID(), issueTxErr
}
func getSignedTx(txEncoded string) (*txs.Tx, error) {
	txBytes, err := formatting.Decode(formatting.Hex, txEncoded)
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
func CreateReadOnlyWallet(
	uri string,
	addresses set.Set[ids.ShortID],
	config primary.WalletConfig,
) (pbuilder.Builder, error) {

	ctx := context.Background()

	avaxState, err := primary.FetchState(ctx, uri, addresses)
	if err != nil {
		return nil, err
	}

	//ethAddrs := ethKeychain.EthAddresses()
	//ethState, err := FetchEthState(ctx, uri, ethAddrs)
	//if err != nil {
	//	return nil, err
	//}

	owners, err := platformvm.GetOwners(avaxState.PClient, ctx, config.SubnetIDs, config.ValidationIDs)
	if err != nil {
		return nil, err
	}

	pUTXOs := common.NewChainUTXOs(constants.PlatformChainID, avaxState.UTXOs)
	pBackend := pwallet.NewBackend(avaxState.PCTX, pUTXOs, owners)
	//pClient := p.NewClient(avaxState.PClient, pBackend)
	pBuilder := pbuilder.New(addresses, avaxState.PCTX, pBackend)

	return pBuilder, nil
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
