// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func removeSurroundingParenthesis(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > 0 {
		if string(s[0]) != "(" || string(s[len(s)-1]) != ")" {
			return "", fmt.Errorf("expected esp %q to be surrounded by parenthesis", s)
		}
		s = s[1 : len(s)-1]
	}
	return s, nil
}

func getWords(s string) []string {
	words := []string{}
	word := ""
	insideParenthesis := false
	for _, rune := range s {
		c := string(rune)
		if insideParenthesis {
			if c == ")" {
				words = append(words, word)
				word = ""
				insideParenthesis = false
			} else {
				word += c
			}
			continue
		}
		if c == " " || c == "," || c == "(" {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		}
		if c == " " || c == "," {
			continue
		}
		if c == "(" {
			insideParenthesis = true
			continue
		}
		word += c
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

func getMap(
	types []string,
	params ...interface{},
) []map[string]interface{} {
	r := []map[string]interface{}{}
	for i, t := range types {
		spaceIndex := strings.Index(t, " ")
		commaIndex := strings.Index(t, ",")
		m := map[string]interface{}{}
		if spaceIndex != -1 || commaIndex != -1 {
			// complex type
			m["components"] = getMap(getWords(t), params[i])
			m["internaltype"] = "tuple"
			m["type"] = "tuple"
			m["name"] = ""
		} else {
			name := ""
			if len(params) == 1 {
				rt := reflect.TypeOf(params[0])
				if rt.NumField() == len(types) {
					name = rt.Field(i).Name
				}
			}
			m["internaltype"] = t
			m["type"] = t
			m["name"] = name
		}
		r = append(r, m)
	}
	return r
}

func ParseMethodEsp(
	methodEsp string,
	paid bool,
	view bool,
	params ...interface{},
) (string, string, error) {
	index := strings.Index(methodEsp, "(")
	if index == -1 {
		return methodEsp, "", nil
	}
	methodName := methodEsp[:index]
	methodTypes := methodEsp[index:]
	methodInputs := ""
	methodOutputs := ""
	index = strings.Index(methodTypes, "->")
	if index == -1 {
		methodInputs = methodTypes
	} else {
		methodInputs = methodTypes[:index]
		methodOutputs = methodTypes[index+2:]
	}
	var err error
	methodInputs, err = removeSurroundingParenthesis(methodInputs)
	if err != nil {
		return "", "", err
	}
	methodOutputs, err = removeSurroundingParenthesis(methodOutputs)
	if err != nil {
		return "", "", err
	}
	inputTypes := getWords(methodInputs)
	outputTypes := getWords(methodOutputs)
	inputs := getMap(inputTypes, params...)
	outputs := getMap(outputTypes)
	abiMap := []map[string]interface{}{
		{
			"inputs":          inputs,
			"outputs":         outputs,
			"name":            methodName,
			"statemutability": "nonpayable",
			"type":            "function",
		},
	}
	if paid {
		abiMap[0]["statemutability"] = "payable"
	}
	if view {
		abiMap[0]["statemutability"] = "view"
	}
	abiBytes, err := json.MarshalIndent(abiMap, "", "  ")
	if err != nil {
		return "", "", err
	}
	return methodName, string(abiBytes), nil
}

func TxToMethod(
	rpcURL string,
	prefundedPrivateKey string,
	contractAddress common.Address,
	payment *big.Int,
	methodEsp string,
	params ...interface{},
) error {
	methodName, methodABI, err := ParseMethodEsp(methodEsp, payment != nil, false, params...)
	if err != nil {
		return err
	}
	metadata := &bind.MetaData{
		ABI: methodABI,
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	contract := bind.NewBoundContract(contractAddress, *abi, client, client, client)
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return err
	}
	txOpts.Value = payment
	tx, err := contract.Transact(txOpts, methodName, params...)
	if err != nil {
		return err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return err
	} else if !success {
		return fmt.Errorf("failed receipt status deploying contract")
	}
	return nil
}

func CallToMethod(
	rpcURL string,
	contractAddress common.Address,
	methodEsp string,
	params ...interface{},
) ([]interface{}, error) {
	methodName, methodABI, err := ParseMethodEsp(methodEsp, false, true, params...)
	if err != nil {
		return nil, err
	}
	metadata := &bind.MetaData{
		ABI: methodABI,
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return nil, err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	contract := bind.NewBoundContract(contractAddress, *abi, client, client, client)
	var out []interface{}
	err = contract.Call(&bind.CallOpts{}, &out, methodName, params...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
