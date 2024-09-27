// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/ictt"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"

	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	ledger "github.com/ava-labs/avalanchego/utils/crypto/ledger"
	avmtxs "github.com/ava-labs/avalanchego/vms/avm/txs"
	goethereumcommon "github.com/ethereum/go-ethereum/common"
)

const (
	cChain                  = "c-chain"
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
		networkoptions.Devnet,
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
	// token transferrer experimental
	originSubnet                  string
	destinationSubnet             string
	originTransferrerAddress      string
	destinationTransferrerAddress string
	destinationKeyName            string
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
		"On what Network do you want to execute the transfer?",
		globalNetworkFlags,
		true,
		false,
		transferSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	subnetNames, err := app.GetBlockchainNamesOnNetwork(network)
	if err != nil {
		return err
	}

	if originSubnet == "" && !PToX && !PToP {
		prompt := "Where are the funds to transfer?"
		cancel, pChainChoosen, _, cChainChoosen, subnetName, _, err := prompts.PromptChain(
			app.Prompt,
			prompt,
			subnetNames,
			false,
			true,
			false,
			"",
			false,
		)
		if err != nil {
			return err
		}
		switch {
		case cancel:
			return nil
		case pChainChoosen:
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
		case cChainChoosen:
			originSubnet = cChain
		default:
			originSubnet = subnetName
		}
	}

	// token transferrer experimental
	if originSubnet != "" {
		if destinationSubnet == "" {
			prompt := "Where are the funds going to?"
			avoidSubnet := originSubnet
			if originSubnet == cChain {
				avoidSubnet = ""
			}
			cancel, _, _, cChainChoosen, subnetName, _, err := prompts.PromptChain(
				app.Prompt,
				prompt,
				subnetNames,
				true,
				true,
				originSubnet == cChain,
				avoidSubnet,
				false,
			)
			if err != nil {
				return err
			}
			switch {
			case cancel:
				return nil
			case cChainChoosen:
				destinationSubnet = cChain
			default:
				destinationSubnet = subnetName
			}
		}
		originURL := network.CChainEndpoint()
		if strings.ToLower(originSubnet) != cChain {
			originURL, _, err = contract.GetBlockchainEndpoints(
				app,
				network,
				contract.ChainSpec{
					BlockchainName: originSubnet,
				},
				true,
				false,
			)
			if err != nil {
				return err
			}
		}
		var destinationBlockchainID ids.ID
		if strings.ToLower(destinationSubnet) == cChain {
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
			amountFlt, err = captureAmount(true, "TOKEN units")
			if err != nil {
				return err
			}
		}
		amount := new(big.Float).SetFloat64(amountFlt)
		amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
		amount = amount.Mul(amount, new(big.Float).SetFloat64(float64(units.Avax)))
		amountInt, _ := amount.Int(nil)
		return ictt.Send(
			originURL,
			goethereumcommon.HexToAddress(originTransferrerAddress),
			privateKey,
			destinationBlockchainID,
			goethereumcommon.HexToAddress(destinationTransferrerAddress),
			destinationAddr,
			amountInt,
		)
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

	if keyName == "" && ledgerIndex == wrongLedgerIndexVal {
		var useLedger bool
		goalStr := ""
		if send {
			goalStr = " for the sender address"
		} else {
			goalStr = " for the destination address"
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

	if amountFlt == 0 {
		amountFlt, err = captureAmount(send, "AVAX units")
		if err != nil {
			return err
		}
	}
	amount := uint64(amountFlt * float64(units.Avax))

	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee

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

	usingLedger := ledgerIndex != wrongLedgerIndexVal

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
		if !usingLedger {
			totalFee = fee
		}
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
		amountPlusFee := amount
		if PToP {
			if usingLedger {
				amountPlusFee += fee * 3
			}
		}
		if PToX {
			amountPlusFee += fee
		}
		output := &avax.TransferableOutput{
			Asset: avax.Asset{ID: wallet.P().Builder().Context().AVAXAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt:          amountPlusFee,
				OutputOwners: to,
			},
		}
		outputs := []*avax.TransferableOutput{output}
		var unsignedTx txs.UnsignedTx
		if PToP && !usingLedger {
			ux.Logger.PrintToUser("Issuing BaseTx P -> P")
			unsignedTx, err = wallet.P().Builder().NewBaseTx(
				outputs,
			)
			if err != nil {
				return fmt.Errorf("error building tx: %w", err)
			}
		} else {
			ux.Logger.PrintToUser("Issuing ExportTx P -> X")
			if usingLedger {
				ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to X Chain' transaction on the ledger device *** ")
			}
			unsignedTx, err = wallet.P().Builder().NewExportTx(
				wallet.X().Builder().Context().BlockchainID,
				outputs,
			)
			if err != nil {
				return fmt.Errorf("error building tx: %w", err)
			}
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
			if usingLedger {
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
				usingLedger,
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
				usingLedger,
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

func captureAmount(sending bool, tokenDesc string) (float64, error) {
	var promptStr string
	if sending {
		promptStr = fmt.Sprintf("Amount to send (%s)", tokenDesc)
	} else {
		promptStr = fmt.Sprintf("Amount to receive (%s)", tokenDesc)
	}
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
