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
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

var ErrFailedReceiptStatus = fmt.Errorf("failed receipt status")

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

func removeSurroundingBrackets(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > 0 {
		if string(s[0]) != "[" || string(s[len(s)-1]) != "]" {
			return "", fmt.Errorf("expected esp %q to be surrounded by parenthesis", s)
		}
		s = s[1 : len(s)-1]
	}
	return s, nil
}

func getWords(s string) []string {
	words := []string{}
	word := ""
	parenthesisCount := 0
	insideBrackets := false
	for _, rune := range s {
		c := string(rune)
		if parenthesisCount > 0 {
			word += c
			if c == "(" {
				parenthesisCount++
			}
			if c == ")" {
				parenthesisCount--
				if parenthesisCount == 0 {
					words = append(words, word)
					word = ""
				}
			}
			continue
		}
		if insideBrackets {
			word += c
			if c == "]" {
				words = append(words, word)
				word = ""
				insideBrackets = false
			}
			continue
		}
		if c == " " || c == "," || c == "(" || c == "[" {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		}
		if c == " " || c == "," {
			continue
		}
		if c == "(" {
			parenthesisCount++
		}
		if c == "[" {
			insideBrackets = true
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
	params interface{},
) ([]map[string]interface{}, error) {
	r := []map[string]interface{}{}
	for i, t := range types {
		var (
			param interface{}
			name  string
		)
		rt := reflect.ValueOf(params)
		if rt.Kind() == reflect.Slice {
			if rt.Len() != len(types) {
				return nil, fmt.Errorf(
					"inconsistency in slice len between method esp %q and given params %#v: expected %d got %d",
					types,
					params,
					len(types),
					rt.Len(),
				)
			}
			param = rt.Index(i).Interface()
		} else if rt.Kind() == reflect.Struct {
			if rt.NumField() != len(types) {
				return nil, fmt.Errorf(
					"inconsistency in struct len between method esp %q and given params %#v: expected %d got %d",
					types,
					params,
					len(types),
					rt.NumField(),
				)
			}
			name = rt.Type().Field(i).Name
			param = rt.Field(i).Interface()
		}
		m := map[string]interface{}{}
		switch {
		case string(t[0]) == "(":
			// struct type
			var err error
			t, err = removeSurroundingParenthesis(t)
			if err != nil {
				return nil, err
			}
			m["components"], err = getMap(getWords(t), param)
			if err != nil {
				return nil, err
			}
			m["internaltype"] = "tuple"
			m["type"] = "tuple"
			m["name"] = name
		case string(t[0]) == "[":
			var err error
			t, err = removeSurroundingBrackets(t)
			if err != nil {
				return nil, err
			}
			if string(t[0]) == "(" {
				t, err = removeSurroundingParenthesis(t)
				if err != nil {
					return nil, err
				}
				rt := reflect.ValueOf(param)
				if rt.Kind() != reflect.Slice {
					return nil, fmt.Errorf("expected param for field %d of esp %q to be an slice", i, types)
				}
				if rt.Len() > i {
					param = rt.Index(0).Interface()
				}
				m["components"], err = getMap(getWords(t), param)
				if err != nil {
					return nil, err
				}
				m["internaltype"] = "tuple[]"
				m["type"] = "tuple[]"
				m["name"] = name
			} else {
				m["internaltype"] = fmt.Sprintf("%s[]", t)
				m["type"] = fmt.Sprintf("%s[]", t)
				m["name"] = name
			}
		default:
			m["internaltype"] = t
			m["type"] = t
			m["name"] = name
		}
		r = append(r, m)
	}
	return r, nil
}

func ParseMethodEsp(
	methodEsp string,
	constructor bool,
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
	inputs, err := getMap(inputTypes, params)
	if err != nil {
		return "", "", err
	}
	outputs, err := getMap(outputTypes, nil)
	if err != nil {
		return "", "", err
	}
	abiMap := []map[string]interface{}{
		{
			"inputs":          inputs,
			"stateMutability": "nonpayable",
			"type":            "function",
		},
	}
	if !constructor {
		abiMap[0]["outputs"] = outputs
		abiMap[0]["name"] = methodName
	} else {
		abiMap[0]["type"] = "constructor"
	}
	if paid {
		abiMap[0]["stateMutability"] = "payable"
	}
	if view {
		abiMap[0]["stateMutability"] = "view"
	}
	abiBytes, err := json.MarshalIndent(abiMap, "", "  ")
	if err != nil {
		return "", "", err
	}
	return methodName, string(abiBytes), nil
}

func TxToMethod(
	rpcURL string,
	privateKey string,
	contractAddress common.Address,
	payment *big.Int,
	methodEsp string,
	params ...interface{},
) (*types.Transaction, *types.Receipt, error) {
	methodName, methodABI, err := ParseMethodEsp(methodEsp, false, payment != nil, false, params...)
	if err != nil {
		return nil, nil, err
	}
	metadata := &bind.MetaData{
		ABI: methodABI,
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return nil, nil, err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()
	contract := bind.NewBoundContract(contractAddress, *abi, client, client, client)
	txOpts, err := evm.GetTxOptsWithSigner(client, privateKey)
	if err != nil {
		return nil, nil, err
	}
	txOpts.Value = payment
	tx, err := contract.Transact(txOpts, methodName, params...)
	if err != nil {
		return nil, nil, err
	}
	receipt, success, err := evm.WaitForTransaction(client, tx)
	if err != nil {
		return tx, nil, err
	} else if !success {
		return tx, receipt, ErrFailedReceiptStatus
	}
	return tx, receipt, nil
}

func CallToMethod(
	rpcURL string,
	contractAddress common.Address,
	methodEsp string,
	params ...interface{},
) ([]interface{}, error) {
	methodName, methodABI, err := ParseMethodEsp(methodEsp, false, false, true, params...)
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

func DeployContract(
	rpcURL string,
	privateKey string,
	binBytes []byte,
	methodEsp string,
	params ...interface{},
) (common.Address, error) {
	_, methodABI, err := ParseMethodEsp(methodEsp, true, false, false, params...)
	if err != nil {
		return common.Address{}, err
	}
	metadata := &bind.MetaData{
		ABI: methodABI,
		Bin: string(binBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return common.Address{}, err
	}
	bin := common.FromHex(metadata.Bin)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return common.Address{}, err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, privateKey)
	if err != nil {
		return common.Address{}, err
	}
	address, tx, _, err := bind.DeployContract(txOpts, *abi, bin, client, params...)
	if err != nil {
		return common.Address{}, err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return common.Address{}, err
	} else if !success {
		return common.Address{}, ErrFailedReceiptStatus
	}
	return address, nil
}
