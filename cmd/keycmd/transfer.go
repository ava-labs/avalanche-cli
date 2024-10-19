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
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
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
	"github.com/ava-labs/coreth/plugin/evm"
	goethereumcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
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
	PToC                bool
	CToP                bool
	CToX                bool
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
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, transferSupportedNetworkOptions)
	cmd.Flags().BoolVar(
		&PToC,
		"fund-c-chain",
		false,
		"fund C-Chain account on destination",
	)
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
		goalStr := "specify the sender address"
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
		amountFlt, err = captureAmount(send, "AVAX units")
		if err != nil {
			return err
		}
	}
	amount := uint64(amountFlt * float64(units.Avax))

	if destinationAddrStr == "" {
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
	}
	if senderChainFlags.CChain && receiverChainFlags.PChain {
		destinationAddr, err := address.ParseToID(destinationAddrStr)
		if err != nil {
			return err
		}
		_ = destinationAddr
	}
	if senderChainFlags.PChain && receiverChainFlags.XChain {
		destinationAddr, err := address.ParseToID(destinationAddrStr)
		if err != nil {
			return err
		}
		_ = destinationAddr
	}

	return nil

	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee

	var destinationAddr ids.ShortID
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
		if CToP || CToX {
			if sk != nil {
				addrStr = sk.C()
			}
		}
		ux.Logger.PrintToUser(
			"- send %.9f AVAX from %s to destination address %s",
			float64(amount)/float64(units.Avax),
			addrStr,
			destinationAddrStr,
		)
		totalFee := 4 * fee
		if !usingLedger {
			totalFee = fee
		}
		if PToX || PToC || CToP || CToX {
			totalFee = 2 * fee
		}
		ux.Logger.PrintToUser(
			"- take a fee of %.9f AVAX from source address %s",
			float64(totalFee)/float64(units.Avax),
			addrStr,
		)
	} else {
		ux.Logger.PrintToUser(
			"- receive %.9f AVAX at destination address %s",
			float64(amount)/float64(units.Avax),
			destinationAddrStr,
		)
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
		ethKeychain := secp256k1fx.NewKeychain()
		if sk != nil {
			ethKeychain = sk.KeyChain()
		}
		wallet, err := primary.MakeWallet(
			context.Background(),
			&primary.WalletConfig{
				URI:          network.Endpoint,
				AVAXKeychain: kc,
				EthKeychain:  ethKeychain,
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
		if PToX || PToC || CToP || CToX {
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

		if CToP {
			ux.Logger.PrintToUser("Issuing ImportTx C -> P")
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
			newOutputs := []*secp256k1fx.TransferOutput{
				{
					Amt:          amountPlusFee,
					OutputOwners: to,
				},
			}
			unsignedTx, err := wallet.C().Builder().NewExportTx(
				avagoconstants.PlatformChainID,
				newOutputs,
				baseFee,
			)
			if err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return fmt.Errorf("error building tx: %w", err)
			}
			tx := evm.Tx{UnsignedAtomicTx: unsignedTx}
			if err := wallet.C().Signer().SignAtomic(context.Background(), &tx); err != nil {
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
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
				ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
				return err
			}
		} else {
			var unsignedTx txs.UnsignedTx
			switch {
			case PToP && !usingLedger:
				ux.Logger.PrintToUser("Issuing BaseTx P -> P")
				if usingLedger {
					ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to X Chain' transaction on the ledger device *** ")
				}
				unsignedTx, err = wallet.P().Builder().NewBaseTx(
					outputs,
				)
				if err != nil {
					return fmt.Errorf("error building tx: %w", err)
				}
			case PToX || (PToP && usingLedger):
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
			case PToC:
				ux.Logger.PrintToUser("Issuing ExportTx P -> C")
				if usingLedger {
					ux.Logger.PrintToUser("*** Please sign 'Export Tx / P to C Chain' transaction on the ledger device *** ")
				}
				unsignedTx, err = wallet.P().Builder().NewExportTx(
					wallet.C().Builder().Context().BlockchainID,
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
			switch {
			case CToP:
				ux.Logger.PrintToUser("Issuing ImportTx C -> P")
				if usingLedger {
					ux.Logger.PrintToUser("*** Please sign ImportTx transaction on the ledger device *** ")
				}
				unsignedTx, err := wallet.P().Builder().NewImportTx(
					wallet.C().Builder().Context().BlockchainID,
					&to,
				)
				if err != nil {
					ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
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
			case PToP || PToX || CToX:
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
			case PToC:
				ux.Logger.PrintToUser("Issuing ImportTx P -> C")
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
				addr, err := app.Prompt.CaptureAddress(
					"Enter the C-Chain destination address",
				)
				if err != nil {
					return err
				}
				unsignedTx, err := wallet.C().Builder().NewImportTx(
					avagoconstants.PlatformChainID,
					addr,
					baseFee,
				)
				if err != nil {
					ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
					return fmt.Errorf("error building tx: %w", err)
				}
				tx := evm.Tx{UnsignedAtomicTx: unsignedTx}
				if err := wallet.C().Signer().SignAtomic(context.Background(), &tx); err != nil {
					ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
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
					ux.Logger.PrintToUser(logging.LightRed.Wrap("ERROR: restart from this step by using the same command"))
					return err
				}
			}

			if PToX || PToC || CToP || CToX {
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
	destinationAddr, err := address.ParseToID(destinationAddrStr)
	if err != nil {
		return err
	}
	ethKeychain := secp256k1fx.NewKeychain()
	wallet, err := primary.MakeWallet(
		context.Background(),
		&primary.WalletConfig{
			URI:          network.Endpoint,
			AVAXKeychain: kc,
			EthKeychain:  ethKeychain,
		},
	)
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
	return nil
}
