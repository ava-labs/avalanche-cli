// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	clievm "github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/ictt"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/units"
	avmtxs "github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	avagofee "github.com/ava-labs/avalanchego/vms/platformvm/txs/fee"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/ava-labs/coreth/plugin/evm/atomic"
	goethereumcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

const (
	keyNameFlag         = "key"
	ledgerIndexFlag     = "ledger"
	amountFlag          = "amount"
	destinationAddrFlag = "destination-addr"
	wrongLedgerIndexVal = 32768
)

var (
	keyName            string
	ledgerIndex        uint32
	destinationAddrStr string
	amountFlt          float64
	// token transferrer experimental
	originSubnet                  string
	destinationSubnet             string
	originTransferrerAddress      string
	destinationTransferrerAddress string
	destinationKeyName            string
	//
	senderChainFlags   contract.ChainSpec
	receiverChainFlags contract.ChainSpec
)

func newTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [options]",
		Short: "Fund a ledger address or stored key from another one",
		Long:  `The key transfer command allows to transfer funds between stored keys or ledger addresses.`,
		RunE:  transferF,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, networkoptions.DefaultSupportedNetworkOptions)
	cmd.Flags().StringVarP(
		&keyName,
		keyNameFlag,
		"k",
		"",
		"key associated to the sender or receiver address",
	)
	cmd.Flags().Uint32VarP(
		&ledgerIndex,
		ledgerIndexFlag,
		"i",
		wrongLedgerIndexVal,
		"ledger index associated to the sender or receiver address",
	)
	cmd.Flags().StringVarP(
		&destinationAddrStr,
		destinationAddrFlag,
		"a",
		"",
		"destination address",
	)
	cmd.Flags().StringVar(
		&destinationKeyName,
		"destination-key",
		"",
		"key associated to a destination address",
	)
	cmd.Flags().Float64VarP(
		&amountFlt,
		amountFlag,
		"o",
		0,
		"amount to send or receive (AVAX or TOKEN units)",
	)
	cmd.Flags().StringVar(
		&originSubnet,
		"origin-subnet",
		"",
		"subnet where the funds belong (token transferrer experimental)",
	)
	cmd.Flags().StringVar(
		&destinationSubnet,
		"destination-subnet",
		"",
		"subnet where the funds will be sent (token transferrer experimental)",
	)
	cmd.Flags().StringVar(
		&originTransferrerAddress,
		"origin-transferrer-address",
		"",
		"token transferrer address at the origin subnet (token transferrer experimental)",
	)
	cmd.Flags().StringVar(
		&destinationTransferrerAddress,
		"destination-transferrer-address",
		"",
		"token transferrer address at the destination subnet (token transferrer experimental)",
	)
	senderChainFlags.SetFlagNames(
		"sender-blockchain",
		"c-chain-sender",
		"p-chain-sender",
		"x-chain-sender",
		"sender-blockchain-id",
	)
	senderChainFlags.AddToCmd(cmd, "send from %s")
	receiverChainFlags.SetFlagNames(
		"receiver-blockchain",
		"c-chain-receiver",
		"p-chain-receiver",
		"x-chain-receiver",
		"receiver-blockchain-id",
	)
	receiverChainFlags.AddToCmd(cmd, "receive at %s")
	return cmd
}

func transferF(*cobra.Command, []string) error {
	if keyName != "" && ledgerIndex != wrongLedgerIndexVal {
		return fmt.Errorf("only one between a keyname or a ledger index must be given")
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to execute the transfer?",
		globalNetworkFlags,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if !senderChainFlags.Defined() {
		prompt := "Where are the funds to transfer?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			"",
			&senderChainFlags,
		); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	if !receiverChainFlags.Defined() {
		prompt := "Where are the funds going to?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			"",
			&receiverChainFlags,
		); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}

	if (senderChainFlags.CChain && receiverChainFlags.CChain) ||
		(senderChainFlags.BlockchainName != "" && senderChainFlags.BlockchainName == receiverChainFlags.BlockchainName) {
		return intraEvmSend(network, senderChainFlags)
	}

	if !senderChainFlags.PChain && !senderChainFlags.XChain && !receiverChainFlags.PChain && !receiverChainFlags.XChain {
		return interEvmSend(network, senderChainFlags, receiverChainFlags)
	}

	senderDesc, err := contract.GetBlockchainDesc(senderChainFlags)
	if err != nil {
		return err
	}
	receiverDesc, err := contract.GetBlockchainDesc(receiverChainFlags)
	if err != nil {
		return err
	}
	if senderChainFlags.BlockchainName != "" || receiverChainFlags.BlockchainName != "" || senderChainFlags.XChain {
		return fmt.Errorf("tranfer from %s to %s is not supported", senderDesc, receiverDesc)
	}

	if keyName == "" && ledgerIndex == wrongLedgerIndexVal {
		var useLedger bool
		goalStr := "as the sender address"
		if receiverChainFlags.XChain {
			ux.Logger.PrintToUser("P->X transfer is an intra-account operation.")
			ux.Logger.PrintToUser("Tokens will be transferred to the same account address on the other chain")
			goalStr = "specify the sender/receiver address"
		}
		if senderChainFlags.CChain && receiverChainFlags.PChain {
			ux.Logger.PrintToUser("C->P transfer is an intra-account operation.")
			ux.Logger.PrintToUser("Tokens will be transferred to the same account address on the other chain")
			goalStr = "as the sender/receiver address"
		}
		useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, goalStr, app.GetKeyDir(), true)
		if err != nil {
			return err
		}
		if useLedger {
			ledgerIndex, err = app.Prompt.CaptureUint32("Ledger index to use")
			if err != nil {
				return err
			}
		}
	}

	var kc keychain.Keychain
	var sk *key.SoftKey
	if keyName != "" {
		sk, err = app.GetKey(keyName, network, false)
		if err != nil {
			return err
		}
		kc = sk.KeyChain()
	} else {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return err
		}
		ledgerIndices := []uint32{ledgerIndex}
		kc, err = keychain.NewLedgerKeychainFromIndices(ledgerDevice, ledgerIndices)
		if err != nil {
			return err
		}
	}
	usingLedger := ledgerIndex != wrongLedgerIndexVal

	if amountFlt == 0 {
		amountFlt, err = captureAmount("AVAX units")
		if err != nil {
			return err
		}
	}
	amount := uint64(amountFlt * float64(units.Avax))

	if destinationAddrStr == "" && !receiverChainFlags.XChain &&
		!(senderChainFlags.CChain && receiverChainFlags.PChain) {
		format := prompts.EVMFormat
		if receiverChainFlags.PChain {
			format = prompts.PChainFormat
		}
		if receiverChainFlags.XChain {
			format = prompts.XChainFormat
		}
		destinationAddrStr, err = prompts.PromptAddress(
			app.Prompt,
			"destination address",
			app.GetKeyDir(),
			app.GetKey,
			"",
			network,
			format,
			"destination address",
		)
		if err != nil {
			return err
		}
	}

	if senderChainFlags.PChain && receiverChainFlags.PChain {
		return pToPSend(
			network,
			kc,
			usingLedger,
			destinationAddrStr,
			amount,
		)
	}

	if senderChainFlags.PChain && receiverChainFlags.CChain {
		return pToCSend(
			network,
			kc,
			usingLedger,
			destinationAddrStr,
			amount,
		)
	}
	if senderChainFlags.CChain && receiverChainFlags.PChain {
		return cToPSend(
			network,
			kc,
			sk,
			usingLedger,
			amount,
		)
	}
	if senderChainFlags.PChain && receiverChainFlags.XChain {
		return pToXSend(
			network,
			kc,
			usingLedger,
			amount,
		)
	}

	return nil
}

func captureAmount(tokenDesc string) (float64, error) {
	promptStr := fmt.Sprintf("Amount to send (%s)", tokenDesc)
	amountFlt, err := app.Prompt.CaptureFloat(promptStr, func(v float64) error {
		if v <= 0 {
			return fmt.Errorf("value %f must be greater than zero", v)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return amountFlt, nil
}

func intraEvmSend(
	network models.Network,
	senderChain contract.ChainSpec,
) error {
	privateKey, err := prompts.PromptPrivateKey(
		app.Prompt,
		"sender private key",
		app.GetKeyDir(),
		app.GetKey,
		"",
		"",
	)
	if err != nil {
		return err
	}
	destinationAddr, err := prompts.PromptAddress(
		app.Prompt,
		"destination address",
		app.GetKeyDir(),
		app.GetKey,
		"",
		network,
		prompts.EVMFormat,
		"destination address",
	)
	if err != nil {
		return err
	}
	amountFlt, err := app.Prompt.CaptureFloat(
		"Amount to transfer",
		func(f float64) error {
			if f <= 0 {
				return fmt.Errorf("not positive")
			}
			return nil
		},
	)
	if err != nil {
		return err
	}
	amountBigFlt := new(big.Float).SetFloat64(amountFlt)
	amountBigFlt = amountBigFlt.Mul(amountBigFlt, new(big.Float).SetInt(vm.OneAvax))
	amount, _ := amountBigFlt.Int(nil)
	senderURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		senderChain,
		true,
		false,
	)
	if err != nil {
		return err
	}
	client, err := clievm.GetClient(senderURL)
	if err != nil {
		return err
	}
	return clievm.FundAddress(client, privateKey, destinationAddr, amount)
}

func interEvmSend(
	network models.Network,
	senderChain contract.ChainSpec,
	receiverChain contract.ChainSpec,
) error {
	senderURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		senderChain,
		true,
		false,
	)
	if err != nil {
		return err
	}
	receiverBlockchainID, err := contract.GetBlockchainID(
		app,
		network,
		receiverChain,
	)
	if err != nil {
		return err
	}
	if originTransferrerAddress == "" {
		addr, err := app.Prompt.CaptureAddress(
			fmt.Sprintf("Enter the address of the Token Transferrer on %s", originSubnet),
		)
		if err != nil {
			return err
		}
		originTransferrerAddress = addr.Hex()
	} else {
		if err := prompts.ValidateAddress(originTransferrerAddress); err != nil {
			return err
		}
	}
	if destinationTransferrerAddress == "" {
		addr, err := app.Prompt.CaptureAddress(
			fmt.Sprintf("Enter the address of the Token Transferrer on %s", destinationSubnet),
		)
		if err != nil {
			return err
		}
		destinationTransferrerAddress = addr.Hex()
	} else {
		if err := prompts.ValidateAddress(destinationTransferrerAddress); err != nil {
			return err
		}
	}
	if keyName == "" {
		keyName, err = prompts.CaptureKeyName(app.Prompt, "fund the transfer", app.GetKeyDir(), true)
		if err != nil {
			return err
		}
	}
	originK, err := app.GetKey(keyName, network, false)
	if err != nil {
		return err
	}
	privateKey := originK.PrivKeyHex()
	var destinationAddr goethereumcommon.Address
	if destinationAddrStr == "" && destinationKeyName == "" {
		option, err := app.Prompt.CaptureList(
			"Do you want to choose a stored key for the destination, or input a destination address?",
			[]string{"Key", "Address"},
		)
		if err != nil {
			return err
		}
		switch option {
		case "Key":
			destinationKeyName, err = prompts.CaptureKeyName(app.Prompt, "receive the transfer", app.GetKeyDir(), true)
			if err != nil {
				return err
			}
		case "Address":
			addr, err := app.Prompt.CaptureAddress(
				"Enter the destination address",
			)
			if err != nil {
				return err
			}
			destinationAddrStr = addr.Hex()
		}
	}
	switch {
	case destinationAddrStr != "":
		if err := prompts.ValidateAddress(destinationAddrStr); err != nil {
			return err
		}
		destinationAddr = goethereumcommon.HexToAddress(destinationAddrStr)
	case destinationKeyName != "":
		destinationK, err := app.GetKey(destinationKeyName, network, false)
		if err != nil {
			return err
		}
		destinationAddrStr = destinationK.C()
		destinationAddr = goethereumcommon.HexToAddress(destinationAddrStr)
	default:
		return fmt.Errorf("you should set the destination address or destination key")
	}
	if amountFlt == 0 {
		amountFlt, err = captureAmount("TOKEN units")
		if err != nil {
			return err
		}
	}
	amount := new(big.Float).SetFloat64(amountFlt)
	amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
	amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
	amountInt, _ := amount.Int(nil)
	return ictt.Send(
		senderURL,
		goethereumcommon.HexToAddress(originTransferrerAddress),
		privateKey,
		receiverBlockchainID,
		goethereumcommon.HexToAddress(destinationTransferrerAddress),
		destinationAddr,
		amountInt,
	)
}

func pToPSend(
	network models.Network,
	kc keychain.Keychain,
	usingLedger bool,
	destinationAddrStr string,
	amount uint64,
) error {
	ethKeychain := secp256k1fx.NewKeychain()
	wallet, err := primary.MakeWallet(
		context.Background(),
		network.Endpoint,
		kc,
		ethKeychain,
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	destinationAddr, err := address.ParseToID(destinationAddrStr)
	if err != nil {
		return err
	}
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{destinationAddr},
	}
	output := &avax.TransferableOutput{
		Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt:          amount,
			OutputOwners: to,
		},
	}
	outputs := []*avax.TransferableOutput{output}
	ux.Logger.PrintToUser("Issuing BaseTx P -> P")
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to X Chain' transaction on the ledger device *** ")
	}
	unsignedTx, err := wallet.P().Builder().NewBaseTx(
		outputs,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.P().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	pContext := wallet.P().Builder().Context()
	pFeeCalculator := avagofee.NewDynamicCalculator(pContext.ComplexityWeights, pContext.GasPrice)
	txFee, err := pFeeCalculator.CalculateFee(unsignedTx)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Paid fee: %.9f", float64(txFee)/float64(units.Avax))
	return nil
}

func pToXSend(
	network models.Network,
	kc keychain.Keychain,
	usingLedger bool,
	amount uint64,
) error {
	ethKeychain := secp256k1fx.NewKeychain()
	wallet, err := primary.MakeWallet(
		context.Background(),
		network.Endpoint,
		kc,
		ethKeychain,
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     kc.Addresses().List(),
	}
	if err := exportFromP(
		amount,
		wallet,
		wallet.X().Builder().Context().BlockchainID,
		"X",
		to,
		usingLedger,
	); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	return importIntoX(
		wallet,
		avagoconstants.PlatformChainID,
		"P",
		to,
		usingLedger,
	)
}

func exportFromP(
	amount uint64,
	wallet *primary.Wallet,
	blockchainID ids.ID,
	blockchainAlias string,
	to secp256k1fx.OutputOwners,
	usingLedger bool,
) error {
	output := &avax.TransferableOutput{
		Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt:          amount,
			OutputOwners: to,
		},
	}
	outputs := []*avax.TransferableOutput{output}
	ux.Logger.PrintToUser("Issuing ExportTx P -> %s", blockchainAlias)
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to %s Chain' transaction on the ledger device *** ", blockchainAlias)
	}
	unsignedTx, err := wallet.P().Builder().NewExportTx(
		blockchainID,
		outputs,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.P().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	return nil
}

func importIntoX(
	wallet *primary.Wallet,
	blockchainID ids.ID,
	blockchainAlias string,
	to secp256k1fx.OutputOwners,
	usingLedger bool,
) error {
	ux.Logger.PrintToUser("Issuing ImportTx %s -> X", blockchainAlias)
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
	}
	unsignedTx, err := wallet.X().Builder().NewImportTx(
		blockchainID,
		&to,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := avmtxs.Tx{Unsigned: unsignedTx}
	if err := wallet.X().Signer().Sign(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.X().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	return nil
}

func pToCSend(
	network models.Network,
	kc keychain.Keychain,
	usingLedger bool,
	destinationAddrStr string,
	amount uint64,
) error {
	ethKeychain := secp256k1fx.NewKeychain()
	wallet, err := primary.MakeWallet(
		context.Background(),
		network.Endpoint,
		kc,
		ethKeychain,
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     kc.Addresses().List(),
	}
	if err := exportFromP(
		amount,
		wallet,
		wallet.C().Builder().Context().BlockchainID,
		"C",
		to,
		usingLedger,
	); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	if err != nil {
		return err
	}
	return importIntoC(
		network,
		wallet,
		avagoconstants.PlatformChainID,
		"P",
		destinationAddrStr,
		usingLedger,
	)
}

func importIntoC(
	network models.Network,
	wallet *primary.Wallet,
	blockchainID ids.ID,
	blockchainAlias string,
	destinationAddrStr string,
	usingLedger bool,
) error {
	ux.Logger.PrintToUser("Issuing ImportTx %s -> C", blockchainAlias)
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
	}
	client, err := clievm.GetClient(network.BlockchainEndpoint("C"))
	if err != nil {
		return err
	}
	baseFee, err := clievm.EstimateBaseFee(client)
	if err != nil {
		return err
	}
	unsignedTx, err := wallet.C().Builder().NewImportTx(
		blockchainID,
		goethereumcommon.HexToAddress(destinationAddrStr),
		baseFee,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := atomic.Tx{UnsignedAtomicTx: unsignedTx}
	if err := wallet.C().Signer().SignAtomic(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.C().IssueAtomicTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	return nil
}

func cToPSend(
	network models.Network,
	kc keychain.Keychain,
	sk *key.SoftKey,
	usingLedger bool,
	amount uint64,
) error {
	ethKeychain := sk.KeyChain()
	wallet, err := primary.MakeWallet(
		context.Background(),
		network.Endpoint,
		kc,
		ethKeychain,
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     kc.Addresses().List(),
	}
	if err := exportFromC(
		network,
		amount,
		wallet,
		avagoconstants.PlatformChainID,
		"P",
		to,
		usingLedger,
	); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	wallet, err = primary.MakeWallet(
		context.Background(),
		network.Endpoint,
		kc,
		ethKeychain,
		primary.WalletConfig{},
	)
	if err != nil {
		return err
	}
	return importIntoP(
		wallet,
		wallet.C().Builder().Context().BlockchainID,
		"C",
		to,
		usingLedger,
	)
}

func exportFromC(
	network models.Network,
	amount uint64,
	wallet *primary.Wallet,
	blockchainID ids.ID,
	blockchainAlias string,
	to secp256k1fx.OutputOwners,
	usingLedger bool,
) error {
	ux.Logger.PrintToUser("Issuing ExportTx C -> %s", blockchainAlias)
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign ExportTx transaction on the ledger device *** ")
	}
	client, err := clievm.GetClient(network.BlockchainEndpoint("C"))
	if err != nil {
		return err
	}
	baseFee, err := clievm.EstimateBaseFee(client)
	if err != nil {
		return err
	}
	outputs := []*secp256k1fx.TransferOutput{
		{
			Amt:          amount,
			OutputOwners: to,
		},
	}
	unsignedTx, err := wallet.C().Builder().NewExportTx(
		blockchainID,
		outputs,
		baseFee,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := atomic.Tx{UnsignedAtomicTx: unsignedTx}
	if err := wallet.C().Signer().SignAtomic(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.C().IssueAtomicTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	return nil
}

func importIntoP(
	wallet *primary.Wallet,
	blockchainID ids.ID,
	blockchainAlias string,
	to secp256k1fx.OutputOwners,
	usingLedger bool,
) error {
	ux.Logger.PrintToUser("Issuing ImportTx %s -> P", blockchainAlias)
	if usingLedger {
		ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
	}
	unsignedTx, err := wallet.P().Builder().NewImportTx(
		blockchainID,
		&to,
	)
	if err != nil {
		return fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	if err := wallet.P().Signer().Sign(context.Background(), &tx); err != nil {
		return fmt.Errorf("error signing tx: %w", err)
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	err = wallet.P().IssueTx(
		&tx,
		common.WithContext(ctx),
	)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timeout issuing/verifying tx with ID %s: %w", tx.ID(), err)
		} else {
			err = fmt.Errorf("error issuing tx with ID %s: %w", tx.ID(), err)
		}
		return err
	}
	return nil
}
