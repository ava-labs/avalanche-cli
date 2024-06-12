// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/bridge"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	avmtxs "github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	goethereumcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

const (
	sendFlag                = "send"
	receiveFlag             = "receive"
	keyNameFlag             = "key"
	ledgerIndexFlag         = "ledger"
	destinationAddrFlag     = "destination-addr"
	amountFlag              = "amount"
	wrongLedgerIndexVal     = 32768
	receiveRecoveryStepFlag = "receive-recovery-step"
)

var (
	transferSupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Mainnet,
		networkoptions.Fuji,
		networkoptions.Local,
	}
	send                bool
	receive             bool
	keyName             string
	ledgerIndex         uint32
	force               bool
	destinationAddrStr  string
	amountFlt           float64
	receiveRecoveryStep uint64
	PToX                bool
	PToP                bool
	// bridge experimental
	originSubnet             string
	destinationSubnet        string
	originBridgeAddress      string
	destinationBridgeAddress string
	destinationKeyName       string
)

func newTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [options]",
		Short: "Fund a ledger address or stored key from another one",
		Long:  `The key transfer command allows to transfer funds between stored keys or ledger addresses.`,
		RunE:  transferF,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, transferSupportedNetworkOptions)
	cmd.Flags().BoolVar(
		&PToX,
		"fund-x-chain",
		false,
		"fund X-Chain account on destination",
	)
	cmd.Flags().BoolVar(
		&PToP,
		"fund-p-chain",
		false,
		"fund P-Chain account on destination",
	)
	cmd.Flags().BoolVar(
		&force,
		forceFlag,
		false,
		"avoid transfer confirmation",
	)
	cmd.Flags().BoolVarP(
		&send,
		sendFlag,
		"s",
		false,
		"send the transfer",
	)
	cmd.Flags().BoolVarP(
		&receive,
		receiveFlag,
		"g",
		false,
		"receive the transfer",
	)
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
	cmd.Flags().Uint64VarP(
		&receiveRecoveryStep,
		receiveRecoveryStepFlag,
		"r",
		0,
		"receive step to use for multiple step transaction recovery",
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
		"subnet where the funds belong (bridge experimental)",
	)
	cmd.Flags().StringVar(
		&destinationSubnet,
		"destination-subnet",
		"",
		"subnet where the funds will be sent (bridge experimental)",
	)
	cmd.Flags().StringVar(
		&originBridgeAddress,
		"origin-bridge-address",
		"",
		"bridge address at the origin subnet (bridge experimental)",
	)
	cmd.Flags().StringVar(
		&destinationBridgeAddress,
		"destination-bridge-address",
		"",
		"bridge address at the destination subnet (bridge experimental)",
	)
	return cmd
}

func transferF(*cobra.Command, []string) error {
	if send && receive {
		return fmt.Errorf("only one of %s, %s flags should be selected", sendFlag, receiveFlag)
	}

	if keyName != "" && ledgerIndex != wrongLedgerIndexVal {
		return fmt.Errorf("only one between a keyname or a ledger index must be given")
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		transferSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	// bridge experimental
	// bridge hub -> spoke
	if originSubnet != "" {
		originURL := network.CChainEndpoint()
		if strings.ToLower(originSubnet) != "c-chain" {
			sc, err := app.LoadSidecar(originSubnet)
			if err != nil {
				return err
			}
			blockchainID := sc.Networks[network.Name()].BlockchainID
			if blockchainID == ids.Empty {
				return fmt.Errorf("subnet %s is not deployed to %s", originSubnet, network.Name())
			}
			originURL = network.BlockchainEndpoint(blockchainID.String())
		}
		if destinationSubnet == "" {
			return fmt.Errorf("you should set destination subnet")
		}
		var destinationBlockchainID ids.ID
		if strings.ToLower(destinationSubnet) == "c-chain" {
			destinationBlockchainID, err = utils.GetChainID(network.Endpoint, "C")
			if err != nil {
				return err
			}
		} else {
			sc, err := app.LoadSidecar(destinationSubnet)
			if err != nil {
				return err
			}
			blockchainID := sc.Networks[network.Name()].BlockchainID
			if blockchainID == ids.Empty {
				return fmt.Errorf("subnet %s is not deployed to %s", destinationSubnet, network.Name())
			}
			destinationBlockchainID = blockchainID
		}
		if originBridgeAddress == "" {
			return fmt.Errorf("you should set bridge address at origin")
		} else {
			if err := prompts.ValidateAddress(originBridgeAddress); err != nil {
				return err
			}
		}
		if destinationBridgeAddress == "" {
			return fmt.Errorf("you should set bridge address at destination")
		} else {
			if err := prompts.ValidateAddress(destinationBridgeAddress); err != nil {
				return err
			}
		}
		if keyName == "" {
			return fmt.Errorf("you should set the key that has the funds")
		}
		originK, err := app.GetKey(keyName, network, false)
		if err != nil {
			return err
		}
		privateKey := originK.PrivKeyHex()
		var destinationAddr goethereumcommon.Address
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
			return fmt.Errorf("you should set the amount")
		}
		amount := new(big.Float).SetFloat64(amountFlt)
		amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
		amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
		amountInt, _ := amount.Int(nil)
		endpointKind, err := bridge.GetEndpointKind(
			originURL,
			goethereumcommon.HexToAddress(originBridgeAddress),
		)
		if err != nil {
			return err
		}
		switch endpointKind {
		case bridge.ERC20TokenSpoke:
			return bridge.ERC20TokenSpokeSend(
				originURL,
				goethereumcommon.HexToAddress(originBridgeAddress),
				privateKey,
				destinationBlockchainID,
				goethereumcommon.HexToAddress(destinationBridgeAddress),
				destinationAddr,
				amountInt,
			)
		case bridge.ERC20TokenHub:
			return bridge.ERC20TokenHubSend(
				originURL,
				goethereumcommon.HexToAddress(originBridgeAddress),
				privateKey,
				destinationBlockchainID,
				goethereumcommon.HexToAddress(destinationBridgeAddress),
				destinationAddr,
				amountInt,
			)
		case bridge.NativeTokenHub:
			return bridge.NativeTokenHubSend(
				originURL,
				goethereumcommon.HexToAddress(originBridgeAddress),
				privateKey,
				destinationBlockchainID,
				goethereumcommon.HexToAddress(destinationBridgeAddress),
				destinationAddr,
				amountInt,
			)
		}
	}

	if !send && !receive {
		option, err := app.Prompt.CaptureList(
			"Step of the transfer",
			[]string{"Send", "Receive"},
		)
		if err != nil {
			return err
		}
		if option == "Send" {
			send = true
		} else {
			receive = true
		}
	}

	if !PToP && !PToX {
		option, err := app.Prompt.CaptureList(
			"Destination Chain",
			[]string{"P-Chain", "X-Chain"},
		)
		if err != nil {
			return err
		}
		if option == "P-Chain" {
			PToP = true
		} else {
			PToX = true
		}
	}

	if keyName == "" && ledgerIndex == wrongLedgerIndexVal {
		var useLedger bool
		goalStr := ""
		if send {
			goalStr = " for the sender address"
		} else {
			goalStr = " for the destination address"
		}
		useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, goalStr, app.GetKeyDir())
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

	if amountFlt == 0 {
		var promptStr string
		if send {
			promptStr = "Amount to send (AVAX units)"
		} else {
			promptStr = "Amount to receive (AVAX units)"
		}
		amountFlt, err = app.Prompt.CaptureFloat(promptStr, func(v float64) error {
			if v <= 0 {
				return fmt.Errorf("value %f must be greater than zero", v)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	amount := uint64(amountFlt * float64(units.Avax))

	fee := network.GenesisParams().TxFee

	var kc keychain.Keychain
	if keyName != "" {
		sk, err := app.GetKey(keyName, network, false)
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

	var destinationAddr ids.ShortID
	if send {
		if destinationAddrStr == "" {
			if PToP {
				destinationAddrStr, err = app.Prompt.CapturePChainAddress("Destination address", network)
				if err != nil {
					return err
				}
			} else {
				destinationAddrStr, err = app.Prompt.CaptureXChainAddress("Destination address", network)
				if err != nil {
					return err
				}
			}
		}
		destinationAddr, err = address.ParseToID(destinationAddrStr)
		if err != nil {
			return err
		}
	} else {
		destinationAddr = kc.Addresses().List()[0]
		destinationAddrStr, err = address.Format("P", key.GetHRP(network.ID), destinationAddr[:])
		if err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("this operation is going to:")
	if send {
		addr := kc.Addresses().List()[0]
		addrStr, err := address.Format("P", key.GetHRP(network.ID), addr[:])
		if err != nil {
			return err
		}
		if addr == destinationAddr && PToP {
			return fmt.Errorf("sender addr is the same as destination addr")
		}
		ux.Logger.PrintToUser("- send %.9f AVAX from %s to destination address %s", float64(amount)/float64(units.Avax), addrStr, destinationAddrStr)
		totalFee := 4 * fee
		if PToX {
			totalFee = 2 * fee
		}
		ux.Logger.PrintToUser("- take a fee of %.9f AVAX from source address %s", float64(totalFee)/float64(units.Avax), addrStr)
	} else {
		ux.Logger.PrintToUser("- receive %.9f AVAX at destination address %s", float64(amount)/float64(units.Avax), destinationAddrStr)
	}
	ux.Logger.PrintToUser("")

	if !force {
		confStr := "Confirm transfer"
		conf, err := app.Prompt.CaptureNoYes(confStr)
		if err != nil {
			return err
		}
		if !conf {
			ux.Logger.PrintToUser("Cancelled")
			return nil
		}
	}

	to := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{destinationAddr},
	}

	if send {
		wallet, err := primary.MakeWallet(
			context.Background(),
			&primary.WalletConfig{
				URI:          network.Endpoint,
				AVAXKeychain: kc,
				EthKeychain:  secp256k1fx.NewKeychain(),
			},
		)
		if err != nil {
			return err
		}
		amountPlusFee := amount + fee*3
		if PToX {
			amountPlusFee = amount + fee
		}
		output := &avax.TransferableOutput{
			Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt:          amountPlusFee,
				OutputOwners: to,
			},
		}
		outputs := []*avax.TransferableOutput{output}
		ux.Logger.PrintToUser("Issuing ExportTx P -> X")

		if ledgerIndex != wrongLedgerIndexVal {
			ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to X Chain' transaction on the ledger device *** ")
		}
		unsignedTx, err := wallet.P().Builder().NewExportTx(
			wallet.X().Builder().Context().BlockchainID,
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
	} else {
		if receiveRecoveryStep == 0 {
			wallet, err := primary.MakeWallet(
				context.Background(),
				&primary.WalletConfig{
					URI:          network.Endpoint,
					AVAXKeychain: kc,
					EthKeychain:  secp256k1fx.NewKeychain(),
				},
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return err
			}
			ux.Logger.PrintToUser("Issuing ImportTx P -> X")
			if ledgerIndex != wrongLedgerIndexVal {
				ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
			}
			unsignedTx, err := wallet.X().Builder().NewImportTx(
				avagoconstants.PlatformChainID,
				&to,
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return fmt.Errorf("error building tx: %w", err)
			}
			tx := avmtxs.Tx{Unsigned: unsignedTx}
			if err := wallet.X().Signer().Sign(context.Background(), &tx); err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
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
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return err
			}

			if PToX {
				return nil
			}

			time.Sleep(2 * time.Second)
			receiveRecoveryStep++
		}
		if receiveRecoveryStep == 1 {
			wallet, err := primary.MakeWallet(
				context.Background(),
				&primary.WalletConfig{
					URI:          network.Endpoint,
					AVAXKeychain: kc,
					EthKeychain:  secp256k1fx.NewKeychain(),
				},
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			ux.Logger.PrintToUser("Issuing ExportTx X -> P")
			_, err = subnet.IssueXToPExportTx(
				wallet,
				ledgerIndex != wrongLedgerIndexVal,
				true,
				wallet.P().Builder().Context().AVAXAssetID,
				amount+fee*1,
				&to,
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			time.Sleep(2 * time.Second)
			receiveRecoveryStep++
		}
		if receiveRecoveryStep == 2 {
			wallet, err := primary.MakeWallet(
				context.Background(),
				&primary.WalletConfig{
					URI:          network.Endpoint,
					AVAXKeychain: kc,
					EthKeychain:  secp256k1fx.NewKeychain(),
				},
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
			ux.Logger.PrintToUser("Issuing ImportTx X -> P")
			_, err = subnet.IssuePFromXImportTx(
				wallet,
				ledgerIndex != wrongLedgerIndexVal,
				true,
				&to,
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap(fmt.Sprintf("ERROR: restart from this step by using the same command with extra arguments: --%s %d", receiveRecoveryStepFlag, receiveRecoveryStep)))
				return err
			}
		}
	}

	return nil
}
