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
	receiverAddrFlag        = "target-addr"
	amountFlag              = "amount"
	wrongLedgerIndexVal     = 32768
	receiveRecoveryStepFlag = "receive-recovery-step"
)

var (
	transferSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Mainnet, networkoptions.Fuji, networkoptions.Local}
	send                            bool
	receive                         bool
	keyName                         string
	ledgerIndex                     uint32
	force                           bool
	receiverAddrStr                 string
	amountFlt                       float64
	receiveRecoveryStep             uint64
	PToX                            bool
	PToP                            bool
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
		"fund X-Chain account on target",
	)
	cmd.Flags().BoolVar(
		&PToP,
		"fund-p-chain",
		false,
		"fund P-Chain account on target",
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
		&receiverAddrStr,
		receiverAddrFlag,
		"a",
		"",
		"receiver address",
	)
	cmd.Flags().Float64VarP(
		&amountFlt,
		amountFlag,
		"o",
		0,
		"amount to send or receive (AVAX units)",
	)
	return cmd
}

func transferF(*cobra.Command, []string) error {
	chain, err := ids.FromString("nqp2DJw1VbjBhG2mGeH5C1JAshqNcTtpBEkj14kcYiT79Jwwu")
	if err != nil {
		return err
	}
	type t struct {
		A1 [32]byte
		A2 goethereumcommon.Address
		A3 goethereumcommon.Address
		A4 goethereumcommon.Address
		A5 *big.Int
		A6 *big.Int
		A7 *big.Int
		A8 goethereumcommon.Address
	}
	tval := t{
		A1: chain,
		A2: goethereumcommon.HexToAddress("0xb623C4495220C603D0A939D32478F55891a61750"),
		A3: goethereumcommon.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"),
		A4: goethereumcommon.HexToAddress("0x5DB9A7629912EBF95876228C24A848de0bfB43A9"),
		A5: big.NewInt(0),
		A6: big.NewInt(0),
		A7: big.NewInt(250000),
		A8: goethereumcommon.Address{},
	}
	val, _ := new(big.Int).SetString("1000000000000000000000", 10)
	err = contract.TxToMethod(
		"http://127.0.0.1:9650/ext/bc/C/rpc",
		"56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027",
		goethereumcommon.HexToAddress("0x4Ac1d98D9cEF99EC6546dEd4Bd550b0b287aaD6D"),
		val,
		"send((bytes32:A1, address:A2, address:A3, address:A4, uint256:A5, uint256:A6, uint256:A7, address:A8))",
		tval,
	)
	if err != nil {
		return err
	}
	return nil
	val, _ = new(big.Int).SetString("2000000000000000000000", 10)
	err = contract.TxToMethod(
		"http://127.0.0.1:9650/ext/bc/C/rpc",
		"56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027",
		goethereumcommon.HexToAddress("0x5DB9A7629912EBF95876228C24A848de0bfB43A9"),
		nil,
		"approve(address, uint256)->(bool)",
		goethereumcommon.HexToAddress("0xa4DfF80B4a1D748BF28BC4A271eD834689Ea3407"),
		val,
	)
	if err != nil {
		return err
	}
	chain, err = ids.FromString("nqp2DJw1VbjBhG2mGeH5C1JAshqNcTtpBEkj14kcYiT79Jwwu")
	if err != nil {
		return err
	}
	tval = t{
		A1: chain,
		A2: goethereumcommon.HexToAddress("0xb623C4495220C603D0A939D32478F55891a61750"),
		A3: goethereumcommon.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"),
		A4: goethereumcommon.HexToAddress("0x5DB9A7629912EBF95876228C24A848de0bfB43A9"),
		A5: big.NewInt(0),
		A6: big.NewInt(0),
		A7: big.NewInt(250000),
		A8: goethereumcommon.Address{},
	}
	val, _ = new(big.Int).SetString("1000000000000000000000", 10)
	err = contract.TxToMethod(
		"http://127.0.0.1:9650/ext/bc/C/rpc",
		"56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027",
		goethereumcommon.HexToAddress("0xa4DfF80B4a1D748BF28BC4A271eD834689Ea3407"),
		nil,
		"send((bytes32:A1, address:A2, address:A3, address:A4, uint256:A5, uint256:A6, uint256:A7, address:A8), uint256)",
		tval,
		val,
	)
	if err != nil {
		return err
	}
	return nil

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
			goalStr = " for the receiver address"
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

	var receiverAddr ids.ShortID
	if send {
		if receiverAddrStr == "" {
			if PToP {
				receiverAddrStr, err = app.Prompt.CapturePChainAddress("Receiver address", network)
				if err != nil {
					return err
				}
			} else {
				receiverAddrStr, err = app.Prompt.CaptureXChainAddress("Receiver address", network)
				if err != nil {
					return err
				}
			}
		}
		receiverAddr, err = address.ParseToID(receiverAddrStr)
		if err != nil {
			return err
		}
	} else {
		receiverAddr = kc.Addresses().List()[0]
		receiverAddrStr, err = address.Format("P", key.GetHRP(network.ID), receiverAddr[:])
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
		if addr == receiverAddr && PToP {
			return fmt.Errorf("sender addr is the same as receiver addr")
		}
		ux.Logger.PrintToUser("- send %.9f AVAX from %s to target address %s", float64(amount)/float64(units.Avax), addrStr, receiverAddrStr)
		totalFee := 4 * fee
		if PToX {
			totalFee = 2 * fee
		}
		ux.Logger.PrintToUser("- take a fee of %.9f AVAX from source address %s", float64(totalFee)/float64(units.Avax), addrStr)
	} else {
		ux.Logger.PrintToUser("- receive %.9f AVAX at target address %s", float64(amount)/float64(units.Avax), receiverAddrStr)
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
		Addrs:     []ids.ShortID{receiverAddr},
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
