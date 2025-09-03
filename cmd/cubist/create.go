// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cubist

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/wallet/chain/p"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
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
	fmt.Printf("customAddrsSet %s \n", customAddrsSet)
	newPWallet, _, builder, err := CreateReadOnlyWallet("https://api.avax-test.network", customAddrsSet, primary.WalletConfig{})
	if err != nil {
		return err
	}

	owners := &secp256k1fx.OutputOwners{
		Addrs:     []ids.ShortID{destinationAddr},
		Threshold: 1,
		Locktime:  0,
	}
	oldTx, err := builder.NewCreateSubnetTx(owners)
	if err != nil {
		return err
	}
	tx := txs.Tx{Unsigned: oldTx}
	// Serialize the unsigned tx
	//txBytes, err := txs.Codec.Marshal(txs.CodecVersion, tx)
	//if err != nil {
	//	return fmt.Errorf("couldn't marshal signed tx: %w", err)
	//}
	//
	//// Get the encoded (in hex + checksum) unsigned tx
	//txStr, err := formatting.Encode(formatting.Hex, txBytes)
	//if err != nil {
	//	return fmt.Errorf("couldn't encode signed tx: %w", err)
	//}
	// Convert to hex with 0x prefix (equivalent to bufferToHex)
	//txStr := "0x" + hex.EncodeToString(txBytes)

	unsignedBytes, err := txs.Codec.Marshal(txs.CodecVersion, &tx.Unsigned)
	if err != nil {
		return fmt.Errorf("couldn't marshal unsigned tx: %w", err)
	}
	// Convert to hex string WITHOUT "0x" prefix
	txStr := hex.EncodeToString(unsignedBytes)
	txStr = "0x" + txStr

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
		signatureStr, _ := m["signature"].(string)
		fmt.Printf("signature %s \n", signatureStr)

		UseSignature(oldTx, signatureStr, *newPWallet)

		//getPublicKeyFromSignature(signatureStr, &tx)
		//if signatureStr[:2] == "0x" {
		//	signatureStr = signatureStr[2:]
		//}
		//fmt.Printf("signatureStr %s \n", signatureStr)
		//// Decode hex to bytes
		//signatureBytes, err := hex.DecodeString(signatureStr)
		//if err != nil {
		//	log.Fatalf("Failed to decode hex: %v", err)
		//}
		//// You might need to determine the correct credential index
		//// For now, let's try putting it in the first credential
		//cred := &secp256k1fx.Credential{
		//	Sigs: make([][65]byte, 1), // Make sure this matches the number of signatures needed
		//}
		//copy(cred.Sigs[0][:], signatureBytes)
		//
		//signedTx := &txs.Tx{
		//	Unsigned: oldTx,
		//	Creds:    []verify.Verifiable{cred},
		//}
		//
		//// Initialize the transaction
		//if err := signedTx.Initialize(txs.Codec); err != nil {
		//	log.Fatalf("Failed to initialize transaction: %v", err)
		//}
		//fmt.Printf("Transaction ID: %s\n", signedTx.ID())
		//
		//txID, err := issueTx(*newPWallet, signedTx)
		//if err != nil {
		//	return fmt.Errorf("failed to issue tx: %w", err)
		//}
		//fmt.Printf("issued txid %s \n", txID.String())
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

//
//func getPublicKeyFromSignature(signedTxStr string, tx *txs.Tx) {
//	// After getting the signature, verify it's from the right address
//	sigBytes, err := hex.DecodeString(signedTxStr[2:])
//	if err != nil {
//		log.Fatalf("Failed to decode hex: %v", err)
//	}
//
//	// Recover the public key and address from the signature
//	unsignedTxBytes, err := txs.Codec.Marshal(txs.CodecVersion, tx)
//	if err != nil {
//		log.Fatalf("Failed to marshal unsigned tx: %v", err)
//	}
//
//	// Hash the unsigned transaction (this is what was signed)
//	hash := sha256.Sum256(unsignedTxBytes)
//
//	// Recover the public key from the signature
//	// Note: secp256k1 signatures are 65 bytes: [r(32) + s(32) + v(1)]
//	if len(sigBytes) != 65 {
//		log.Fatalf("Invalid signature length: expected 65 bytes, got %d", len(sigBytes))
//	}
//
//	// Extract r, s, v from the signature
//	r := sigBytes[:32]
//	s := sigBytes[32:64]
//	v := sigBytes[64]
//
//	// Recover the public key
//	publicKeyBytes, err := secp256k1.RecoverPubkey(hash[:], append(r, append(s, v)...))
//	if err != nil {
//		log.Fatalf("Failed to recover public key: %v", err)
//	}
//
//	// Convert raw public key bytes to ecdsa.PublicKey
//	// secp256k1 public keys are 65 bytes: [0x04 + x(32) + y(32)]
//	if len(publicKeyBytes) != 65 {
//		log.Fatalf("Invalid public key length: expected 65 bytes, got %d", len(publicKeyBytes))
//	}
//
//	// Skip the first byte (0x04) and extract x, y coordinates
//	x := new(big.Int).SetBytes(publicKeyBytes[1:33])
//	y := new(big.Int).SetBytes(publicKeyBytes[33:65])
//
//	// Create the ecdsa.PublicKey
//	publicKey := &ecdsa.PublicKey{
//		Curve: secp256k1.S256(), // or elliptic.P256() if you don't have secp256k1.S256()
//		X:     x,
//		Y:     y,
//	}
//
//	// Convert public key to address
//	ethAddress := crypto.PubkeyToAddress(*publicKey)
//	fmt.Printf("Signature is from address: %s\n", ethAddress.Hex())
//
//	// Convert Ethereum address to Avalanche P-chain address
//	// Remove 0x prefix from Ethereum address
//	ethAddrStr := ethAddress.Hex()[2:] // Remove "0x"
//
//	// Decode hex to bytes
//	ethAddrBytes, err := hex.DecodeString(ethAddrStr)
//	if err != nil {
//		log.Fatalf("Failed to decode eth address: %v", err)
//	}
//
//	// Convert to Avalanche ShortID
//	avaxAddr := ids.ShortID{}
//	copy(avaxAddr[:], ethAddrBytes)
//
//	// Convert to P-chain address format
//	pChainAddr, err := address.Format("P", "fuji", avaxAddr[:])
//	if err != nil {
//		log.Fatalf("Failed to format P-chain address: %v", err)
//	}
//
//	fmt.Printf("P-chain address: %s\n", pChainAddr)
//
//}

func CreateReadOnlyWallet(
	uri string,
	addresses set.Set[ids.ShortID],
	config primary.WalletConfig,
) (*pwallet.Wallet, *p.Client, pbuilder.Builder, error) {

	ctx, cancel := sdkutils.GetTimedContext(3 * time.Minute)
	defer cancel()

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

// ExternalSigner implements the Signer interface but uses a pre-computed signature
type ExternalSigner struct {
	signature string // The hex signature string you provided
}

// NewExternalSigner creates a new external signer with the provided signature
func NewExternalSigner(signature string) *ExternalSigner {
	return &ExternalSigner{
		signature: signature,
	}
}

// Sign implements the Signer interface
// This method adds the pre-computed signature to the transaction
func (s *ExternalSigner) Sign(ctx context.Context, tx *txs.Tx) error {
	// Remove "0x" prefix if present
	sigStr := s.signature
	if len(sigStr) > 2 && sigStr[:2] == "0x" {
		sigStr = sigStr[2:]
	}

	// Decode hex signature to bytes
	signatureBytes, err := hex.DecodeString(sigStr)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature length (should be 65 bytes for secp256k1)
	if len(signatureBytes) != secp256k1.SignatureLen {
		return fmt.Errorf("invalid signature length: expected %d bytes, got %d", secp256k1.SignatureLen, len(signatureBytes))
	}

	// Create the credential with the signature
	cred := &secp256k1fx.Credential{
		Sigs: make([][secp256k1.SignatureLen]byte, 1),
	}
	copy(cred.Sigs[0][:], signatureBytes)

	// Set the credentials on the transaction
	tx.Creds = []verify.Verifiable{cred}

	// Initialize the transaction to set the TxID and bytes
	if err := tx.Initialize(txs.Codec); err != nil {
		return fmt.Errorf("failed to initialize transaction: %w", err)
	}

	return nil
}

// CreateSignedTransactionDirectly creates a signed transaction without using the Signer interface
// This is the simplest approach for your use case
func CreateSignedTransactionDirectly(unsignedTx txs.UnsignedTx, signature string) (*txs.Tx, error) {
	// Remove "0x" prefix if present
	sigStr := signature
	if len(sigStr) > 2 && sigStr[:2] == "0x" {
		sigStr = sigStr[2:]
	}

	// Decode hex signature to bytes
	signatureBytes, err := hex.DecodeString(sigStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature length
	if len(signatureBytes) != secp256k1.SignatureLen {
		return nil, fmt.Errorf("invalid signature length: expected %d bytes, got %d", secp256k1.SignatureLen, len(signatureBytes))
	}

	// Create the credential with the signature
	cred := &secp256k1fx.Credential{
		Sigs: make([][secp256k1.SignatureLen]byte, 1),
	}
	copy(cred.Sigs[0][:], signatureBytes)

	// Create the signed transaction
	signedTx := &txs.Tx{
		Unsigned: unsignedTx,
		Creds:    []verify.Verifiable{cred},
	}

	// Initialize the transaction
	if err := signedTx.Initialize(txs.Codec); err != nil {
		return nil, fmt.Errorf("failed to initialize transaction: %w", err)
	}

	return signedTx, nil
}

// Example usage function showing how to integrate with your existing code
func UseSignature(unsignedTx txs.UnsignedTx, signature string, newPWallet pwallet.Wallet) {
	// Method 1: Using the ExternalSigner (implements the Signer interface)
	signer := NewExternalSigner(signature)
	tx := &txs.Tx{
		Unsigned: unsignedTx, // Your unsigned transaction
	}

	if err := signer.Sign(context.Background(), tx); err != nil {
		fmt.Printf("Failed to sign transaction: %v\n", err)
		return
	}

	fmt.Printf("Transaction ID: %s\n", tx.ID())

	// Method 2: Direct creation (simpler, no interface needed)
	signedTx, err := CreateSignedTransactionDirectly(unsignedTx, signature)
	if err != nil {
		fmt.Printf("Failed to create signed transaction: %v\n", err)
		return
	}

	fmt.Printf("Transaction ID: %s\n", signedTx.ID())

	// Now you can issue the transaction
	// txID, err := issueTx(*newPWallet, signedTx)
	// if err != nil {
	//     return fmt.Errorf("failed to issue tx: %w", err)
	// }
	txID, err := issueTx(newPWallet, signedTx)
	if err != nil {
		fmt.Printf("failed to issue tx: %w", err)
	}
	fmt.Printf("issued txid %s \n", txID.String())
}
