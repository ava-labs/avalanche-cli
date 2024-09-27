// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"

	_ "embed"
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
			param      interface{}
			name       string
			structName string
		)
		rt := reflect.ValueOf(params)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() == reflect.Slice {
			if rt.Len() != len(types) {
				if rt.Len() == 1 {
					return getMap(types, rt.Index(0).Interface())
				} else {
					return nil, fmt.Errorf(
						"inconsistency in slice len between method esp %q and given params %#v: expected %d got %d",
						types,
						params,
						len(types),
						rt.Len(),
					)
				}
			}
			param = rt.Index(i).Interface()
		} else if rt.Kind() == reflect.Struct {
			if rt.NumField() < len(types) {
				return nil, fmt.Errorf(
					"inconsistency in struct len between method esp %q and given params %#v: expected %d got %d",
					types,
					params,
					len(types),
					rt.NumField(),
				)
			}
			name = rt.Type().Field(i).Name
			structName = rt.Type().Field(i).Type.Name()
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
			if structName != "" {
				m["internalType"] = "struct " + structName
			} else {
				m["internalType"] = "tuple"
			}
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
				param = reflect.Zero(rt.Type().Elem()).Interface()
				structName = rt.Type().Elem().Name()
				m["components"], err = getMap(getWords(t), param)
				if err != nil {
					return nil, err
				}
				if structName != "" {
					m["internalType"] = "struct " + structName + "[]"
				} else {
					m["internalType"] = "tuple[]"
				}
				m["type"] = "tuple[]"
				m["name"] = name
			} else {
				m["internalType"] = fmt.Sprintf("%s[]", t)
				m["type"] = fmt.Sprintf("%s[]", t)
				m["name"] = name
			}
		default:
			m["internalType"] = t
			m["type"] = t
			m["name"] = name
		}
		r = append(r, m)
	}
	return r, nil
}

func ParseSpec(
	esp string,
	indexedFields []int,
	constructor bool,
	event bool,
	paid bool,
	view bool,
	params ...interface{},
) (string, string, error) {
	index := strings.Index(esp, "(")
	if index == -1 {
		return esp, "", nil
	}
	name := esp[:index]
	types := esp[index:]
	inputs := ""
	outputs := ""
	index = strings.Index(types, "->")
	if index == -1 {
		inputs = types
	} else {
		inputs = types[:index]
		outputs = types[index+2:]
	}
	var err error
	inputs, err = removeSurroundingParenthesis(inputs)
	if err != nil {
		return "", "", err
	}
	outputs, err = removeSurroundingParenthesis(outputs)
	if err != nil {
		return "", "", err
	}
	inputTypes := getWords(inputs)
	outputTypes := getWords(outputs)
	inputsMaps, err := getMap(inputTypes, params)
	if err != nil {
		return "", "", err
	}
	outputsMaps, err := getMap(outputTypes, nil)
	if err != nil {
		return "", "", err
	}
	if event {
		for i := range inputsMaps {
			if utils.Belongs(indexedFields, i) {
				inputsMaps[i]["indexed"] = true
			}
		}
	}
	abiMap := []map[string]interface{}{
		{
			"inputs": inputsMaps,
		},
	}
	switch {
	case paid:
		abiMap[0]["stateMutability"] = "payable"
	case view:
		abiMap[0]["stateMutability"] = "view"
	default:
		abiMap[0]["stateMutability"] = "nonpayable"
	}
	switch {
	case constructor:
		abiMap[0]["type"] = "constructor"
	case event:
		abiMap[0]["type"] = "event"
		abiMap[0]["name"] = name
		delete(abiMap[0], "stateMutability")
	default:
		abiMap[0]["type"] = "function"
		abiMap[0]["outputs"] = outputsMaps
		abiMap[0]["name"] = name
	}
	abiBytes, err := json.MarshalIndent(abiMap, "", "  ")
	if err != nil {
		return "", "", err
	}
	return name, string(abiBytes), nil
}

func TxToMethod(
	rpcURL string,
	privateKey string,
	contractAddress common.Address,
	payment *big.Int,
	methodSpec string,
	params ...interface{},
) (*types.Transaction, *types.Receipt, error) {
	methodName, methodABI, err := ParseSpec(methodSpec, nil, false, false, payment != nil, false, params...)
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
	methodSpec string,
	params ...interface{},
) ([]interface{}, error) {
	methodName, methodABI, err := ParseSpec(methodSpec, nil, false, false, false, true, params...)
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
	methodSpec string,
	params ...interface{},
) (common.Address, error) {
	_, methodABI, err := ParseSpec(methodSpec, nil, true, false, false, false, params...)
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

func UnpackLog(
	eventSpec string,
	indexedFields []int,
	log types.Log,
	event interface{},
) error {
	eventName, eventABI, err := ParseSpec(eventSpec, indexedFields, false, true, false, false, event)
	if err != nil {
		return err
	}
	metadata := &bind.MetaData{
		ABI: eventABI,
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return err
	}
	contract := bind.NewBoundContract(common.Address{}, *abi, nil, nil, nil)
	return contract.UnpackLog(event, eventName, log)
}
