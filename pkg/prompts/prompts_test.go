// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prompts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/mocks"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/crypto"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	cliKeyOpt       = "Get private key from an existing stored key (created from avalanche key create or avalanche key import)"
	cliAddrOpt      = "Get address from an existing stored key (created from avalanche key create or avalanche key import)"
	testTransaction = "test transaction"
)

/*
	These tests need to be in an independent module
	because otherwise they create a circular dependency
	between the mocks and the modules
*/

func TestCaptureListDecision(t *testing.T) {
	require := require.New(t)
	mockPrompt := &mocks.Prompter{}

	pk, err := crypto.GenerateKey()
	require.NoError(err)
	addr := crypto.PubkeyToAddress(pk.PublicKey)
	pk2, err := crypto.GenerateKey()
	require.NoError(err)
	addr2 := crypto.PubkeyToAddress(pk2.PublicKey)

	// 1. cancel
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Cancel, nil).Once()

	// 2. error from CaptureList
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("", errors.New("fake error")).Once()

	// 3. error from capture function
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything).Return(common.Address{}, errors.New("capture function error")).Once()

	// 4. add - 1 valid, 1 done
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 5. add - 1 valid, then add the same
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	// do a preview (nothing changes, but prints 2 addrs on STDOUT)
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Preview, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 6. add - 2 valid, then remove index 1, readd
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	// print info (nothing changes, but prints info to STDOUT)
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(MoreInfo, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Del, nil).Once()
	mockPrompt.On("CaptureIndex", mock.Anything, mock.Anything).Return(1, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 7. add 1 item, then delete it (list becomes empty), then done
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Del, nil).Once()
	mockPrompt.On("CaptureIndex", mock.Anything, mock.Anything).Return(0, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 8. try to delete from empty list, then done
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Del, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 9. try to preview empty list, then done
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Preview, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Done, nil).Once()

	// 10. add item, then delete but CaptureIndex returns error
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(Del, nil).Once()
	mockPrompt.On("CaptureIndex", mock.Anything, mock.Anything).Return(0, errors.New("index error")).Once()

	prompt := "Test CaptureListDecision"
	capture := mockPrompt.CaptureAddress
	capturePrompt := "Enter address"
	label := "Test"
	info := "something"

	// 1.cancel
	list, cancel, err := CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.True(cancel)
	require.Empty(list)

	// 2. error from CaptureList
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.Error(err)
	require.ErrorContains(err, "fake error")
	require.False(cancel)
	require.Empty(list)

	// 3. error from capture function
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.Error(err)
	require.ErrorContains(err, "capture function error")
	require.False(cancel)
	require.Empty(list)

	// 4. add - 1 valid, 1 done
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(1, len(list))

	// 5. add - 1 valid, then add the same
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(2, len(list))

	// 6. add - 2 valid, then remove index 1, readd
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(2, len(list))

	// 7. add 1 item, then delete it (list becomes empty), then done
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(0, len(list))

	// 8. try to delete from empty list, then done
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(0, len(list))

	// 9. try to preview empty list, then done
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.NoError(err)
	require.False(cancel)
	require.Exactly(0, len(list))

	// 10. add item, then delete but CaptureIndex returns error
	list, cancel, err = CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
	)
	require.Error(err)
	require.ErrorContains(err, "index error")
	require.False(cancel)
	require.Empty(list)
}

func TestCheckSubnetAuthKeys(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		name           string
		walletKeys     []string
		subnetAuthKeys []string
		controlKeys    []string
		threshold      uint32
		expectedErr    string
	}{
		{
			name:           "valid case",
			walletKeys:     []string{"key1", "key2"},
			subnetAuthKeys: []string{"key1", "key2"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "",
		},
		{
			name:           "wallet key is control key but not in auth keys",
			walletKeys:     []string{"key1", "key2"},
			subnetAuthKeys: []string{"key2"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      1,
			expectedErr:    "wallet key key1 is a control key so it must be included in auth keys",
		},
		{
			name:           "auth keys count doesn't match threshold",
			walletKeys:     []string{"key1", "key2"},
			subnetAuthKeys: []string{"key1"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "wallet key key2 is a control key so it must be included in auth keys",
		},
		{
			name:           "auth key not in control keys",
			walletKeys:     []string{"key1", "key2"},
			subnetAuthKeys: []string{"key1", "key4"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "wallet key key2 is a control key so it must be included in auth keys",
		},
		{
			name:           "auth keys count doesn't match threshold - too few",
			walletKeys:     []string{"key1"},
			subnetAuthKeys: []string{"key1"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "number of given auth keys differs from the threshold",
		},
		{
			name:           "auth keys count doesn't match threshold - too many",
			walletKeys:     []string{"key1"},
			subnetAuthKeys: []string{"key1", "key2", "key3"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "number of given auth keys differs from the threshold",
		},
		{
			name:           "auth key not found in control keys",
			walletKeys:     []string{"key1"},
			subnetAuthKeys: []string{"key1", "key4"},
			controlKeys:    []string{"key1", "key2", "key3"},
			threshold:      2,
			expectedErr:    "auth key key4 does not belong to control keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			err := CheckSubnetAuthKeys(tt.walletKeys, tt.subnetAuthKeys, tt.controlKeys, tt.threshold)
			if tt.expectedErr == "" {
				require.NoError(err)
			} else {
				require.Error(err)
				require.ErrorContains(err, tt.expectedErr)
			}
		})
	}
}

func TestGetSubnetAuthKeys(t *testing.T) {
	require := require.New(t)
	mockPrompt := &mocks.Prompter{}

	t.Run("shortcut returns controlKeys", func(*testing.T) {
		walletKeys := []string{"key1", "key2"}
		controlKeys := []string{"key1", "key2"}
		threshold := uint32(2)
		result, err := GetSubnetAuthKeys(mockPrompt, walletKeys, controlKeys, threshold)
		require.NoError(err)
		require.Equal(controlKeys, result)
	})

	t.Run("interactive adds wallet control keys and for rest", func(t *testing.T) {
		walletKeys := []string{"key1"}
		controlKeys := []string{"key1", "key2", "key3"}
		threshold := uint32(2)
		// key1 is added automatically, user selects key2
		mockPrompt.On("CaptureList", mock.Anything, []string{"key2", "key3"}).Return("key2", nil).Once()
		result, err := GetSubnetAuthKeys(mockPrompt, walletKeys, controlKeys, threshold)
		require.NoError(err)
		require.ElementsMatch([]string{"key1", "key2"}, result)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from user prompt is returned", func(t *testing.T) {
		walletKeys := []string{"key1"}
		controlKeys := []string{"key1", "key2", "key3"}
		threshold := uint32(2)
		mockPrompt.On("CaptureList", mock.Anything, []string{"key2", "key3"}).Return("", errors.New("user error")).Once()
		result, err := GetSubnetAuthKeys(mockPrompt, walletKeys, controlKeys, threshold)
		require.Error(err)
		require.Contains(err.Error(), "user error")
		require.Nil(result)
		mockPrompt.AssertExpectations(t)
	})
}

func TestGetKeyOrLedger(t *testing.T) {
	require := require.New(t)
	goal := "test goal"
	includeEwoq := true

	t.Run("user chooses Ledger", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		mockPrompt.On("ChooseKeyOrLedger", goal).Return(false, nil).Once()
		useLedger, keyName, err := GetKeyOrLedger(mockPrompt, goal, keyDir, includeEwoq)
		require.True(useLedger)
		require.Empty(keyName)
		require.NoError(err)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user chooses stored key and key is found", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "key1")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)
		mockPrompt.On("ChooseKeyOrLedger", goal).Return(true, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("key1", nil).Once()
		useLedger, keyName, err := GetKeyOrLedger(mockPrompt, goal, keyDir, includeEwoq)
		require.False(useLedger)
		require.Equal("key1", keyName)
		require.NoError(err)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user chooses stored key but no keys are found", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		mockPrompt.On("ChooseKeyOrLedger", goal).Return(true, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("should not be called")).Maybe()
		useLedger, keyName, err := GetKeyOrLedger(mockPrompt, goal, keyDir, false)
		require.False(useLedger)
		require.Empty(keyName)
		require.EqualError(err, "no keys")
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user chooses stored key but error occurs", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "key1")
		errSome := errors.New("some error")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)
		mockPrompt.On("ChooseKeyOrLedger", goal).Return(true, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("", errSome).Once()
		useLedger, keyName, err := GetKeyOrLedger(mockPrompt, goal, keyDir, includeEwoq)
		require.False(useLedger)
		require.Empty(keyName)
		require.ErrorIs(err, errSome)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from ChooseKeyOrLedger", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		errChoose := errors.New("choose error")
		mockPrompt.On("ChooseKeyOrLedger", goal).Return(false, errChoose).Once()
		useLedger, keyName, err := GetKeyOrLedger(mockPrompt, goal, keyDir, includeEwoq)
		require.False(useLedger)
		require.Empty(keyName)
		require.ErrorIs(err, errChoose)
		mockPrompt.AssertExpectations(t)
	})
}

func TestCaptureKeyName(t *testing.T) {
	require := require.New(t)
	goal := "test goal"
	includeEwoq := true

	t.Run("error from GetKeyNames (nonexistent dir)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		nonexistentDir := filepath.Join(os.TempDir(), "does-not-exist-1234")
		_, err := CaptureKeyName(mockPrompt, goal, nonexistentDir, includeEwoq)
		require.Error(err)
	})

	t.Run("no keys found (empty dir)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		dir := t.TempDir()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("should not be called")).Maybe()
		_, err := CaptureKeyName(mockPrompt, goal, dir, false) // includeEwoq=false to avoid default key
		require.Error(err)
		require.EqualError(err, "no keys")
	})

	t.Run("user selects a key", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		dir := t.TempDir()
		key1 := filepath.Join(dir, "key1.pk")
		key2 := filepath.Join(dir, "key2.pk")
		err := os.WriteFile(key1, []byte("dummy"), 0o600)
		require.NoError(err)
		err = os.WriteFile(key2, []byte("dummy"), 0o600)
		require.NoError(err)
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.MatchedBy(func(keys []string) bool {
			return len(keys) == 2 && ((keys[0] == "key1" && keys[1] == "key2") || (keys[0] == "key2" && keys[1] == "key1"))
		}), 2).Return("key2", nil).Once()
		keyName, err := CaptureKeyName(mockPrompt, goal, dir, false) // includeEwoq=false to avoid interference
		require.NoError(err)
		require.Equal("key2", keyName)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureListWithSize", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		dir := t.TempDir()
		key1 := filepath.Join(dir, "key1.pk")
		key2 := filepath.Join(dir, "key2.pk")
		err := os.WriteFile(key1, []byte("dummy"), 0o600)
		require.NoError(err)
		err = os.WriteFile(key2, []byte("dummy"), 0o600)
		require.NoError(err)
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, 2).Return("", errors.New("user cancel")).Once()
		_, err = CaptureKeyName(mockPrompt, goal, dir, false) // includeEwoq=false to avoid interference
		require.Error(err)
		require.Contains(err.Error(), "user cancel")
		mockPrompt.AssertExpectations(t)
	})

	t.Run("size capped at 10 when more than 10 keys", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		dir := t.TempDir()

		// Create 12 keys to test size > 10 condition
		keyNames := make([]string, 12)
		for i := 0; i < 12; i++ {
			keyName := fmt.Sprintf("key%d", i+1)
			keyNames[i] = keyName
			keyFile := filepath.Join(dir, keyName+".pk")
			err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
			require.NoError(err)
		}

		// Mock should be called with size=10 (capped) but with all 12 key names
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.MatchedBy(func(keys []string) bool {
			// Verify all 12 keys are passed
			return len(keys) == 12
		}), 10).Return("key5", nil).Once()

		keyName, err := CaptureKeyName(mockPrompt, goal, dir, false) // includeEwoq=false to avoid interference
		require.NoError(err)
		require.Equal("key5", keyName)
		mockPrompt.AssertExpectations(t)
	})
}

func TestCaptureBoolFlag(t *testing.T) {
	require := require.New(t)
	flagName := "test-flag"
	promptMsg := "Do you want to enable this feature?"

	t.Run("flagValue is true (shortcut)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		cmd := &cobra.Command{}
		result, err := CaptureBoolFlag(mockPrompt, cmd, flagName, true, promptMsg)
		require.NoError(err)
		require.True(result)
		// No mock expectations needed since it should return immediately
	})

	t.Run("flag doesn't exist (user)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		cmd := &cobra.Command{}
		mockPrompt.On("CaptureYesNo", promptMsg).Return(true, nil).Once()
		result, err := CaptureBoolFlag(mockPrompt, cmd, flagName, false, promptMsg)
		require.NoError(err)
		require.True(result)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("flag exists but not changed (user)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		cmd := &cobra.Command{}
		cmd.Flags().Bool(flagName, false, "test flag")
		mockPrompt.On("CaptureYesNo", promptMsg).Return(false, nil).Once()
		result, err := CaptureBoolFlag(mockPrompt, cmd, flagName, false, promptMsg)
		require.NoError(err)
		require.False(result)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("flag exists and changed (gets from flags)", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		cmd := &cobra.Command{}
		cmd.Flags().Bool(flagName, false, "test flag")
		err := cmd.Flags().Set(flagName, "true") // This marks the flag as changed
		require.NoError(err)
		result, err := CaptureBoolFlag(mockPrompt, cmd, flagName, false, promptMsg)
		require.NoError(err)
		require.True(result)
		// No mock expectations needed since it gets value from flags
	})

	t.Run("error from CaptureYesNo", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		cmd := &cobra.Command{}
		mockPrompt.On("CaptureYesNo", promptMsg).Return(false, errors.New("user error")).Once()
		result, err := CaptureBoolFlag(mockPrompt, cmd, flagName, false, promptMsg)
		require.Error(err)
		require.Contains(err.Error(), "user error")
		require.False(result)
		mockPrompt.AssertExpectations(t)
	})
}

func TestPromptChain(t *testing.T) {
	require := require.New(t)
	prompt := "Select a chain"
	subnetNames := []string{"subnet1", "subnet2", "subnet3"}
	avoidBlockchainName := "subnet2"

	t.Run("user selects P-Chain", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			return len(options) > 0 && options[0] == "P-Chain"
		}), 11).Return("P-Chain", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.NoError(err)
		require.False(notListed)
		require.True(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects X-Chain", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			return len(options) > 0 && options[1] == "X-Chain"
		}), 11).Return("X-Chain", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.NoError(err)
		require.False(notListed)
		require.False(pChain)
		require.True(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects C-Chain", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			return len(options) > 0 && options[2] == "C-Chain"
		}), 11).Return("C-Chain", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.NoError(err)
		require.False(notListed)
		require.False(pChain)
		require.False(xChain)
		require.True(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects a blockchain", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			// Should contain "Blockchain subnet1" and "Blockchain subnet3" but not "Blockchain subnet2" (avoided)
			hasSubnet1 := false
			hasSubnet3 := false
			hasSubnet2 := false
			for _, opt := range options {
				if opt == "Blockchain subnet1" {
					hasSubnet1 = true
				}
				if opt == "Blockchain subnet3" {
					hasSubnet3 = true
				}
				if opt == "Blockchain subnet2" {
					hasSubnet2 = true
				}
			}
			return hasSubnet1 && hasSubnet3 && !hasSubnet2
		}), 11).Return("Blockchain subnet1", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.NoError(err)
		require.False(notListed)
		require.False(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Equal("subnet1", subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects Custom", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			// Should contain "Custom" option
			for _, opt := range options {
				if opt == Custom {
					return true
				}
			}
			return false
		}), 11).Return(Custom, nil).Once()
		mockPrompt.On("CaptureString", "Blockchain ID/Alias").Return("custom-blockchain-id", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, true)
		require.NoError(err)
		require.False(notListed)
		require.False(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Equal("custom-blockchain-id", blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects My blockchain isn't listed", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.MatchedBy(func(options []string) bool {
			// Should contain "My blockchain isn't listed" option
			for _, opt := range options {
				if opt == "My blockchain isn't listed" {
					return true
				}
			}
			return false
		}), 11).Return("My blockchain isn't listed", nil).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.NoError(err)
		require.True(notListed)
		require.False(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureListWithSize", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.Anything, 11).Return("", errors.New("user error")).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, false)
		require.Error(err)
		require.Contains(err.Error(), "user error")
		require.False(notListed)
		require.False(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureString in Custom", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		mockPrompt.On("CaptureListWithSize", prompt, mock.Anything, 11).Return(Custom, nil).Once()
		mockPrompt.On("CaptureString", "Blockchain ID/Alias").Return("", errors.New("string error")).Once()
		notListed, pChain, xChain, cChain, subnetName, blockchainID, err := PromptChain(
			mockPrompt, prompt, subnetNames, true, true, true, avoidBlockchainName, true)
		require.Error(err)
		require.Contains(err.Error(), "string error")
		require.False(notListed)
		require.False(pChain)
		require.False(xChain)
		require.False(cChain)
		require.Empty(subnetName)
		require.Empty(blockchainID)
		mockPrompt.AssertExpectations(t)
	})
}

func TestPromptPrivateKey(t *testing.T) {
	require := require.New(t)
	goal := testTransaction
	genesisAddress := "0x1234567890123456789012345678901234567890"
	genesisPrivateKey := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	t.Run("user selects CLI key option", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return nil, errors.New("mock key - check parameters only")
		}

		mockPrompt.On("CaptureList", "Which private key do you want to use to test transaction?", expectedOptions).Return(cliKeyOpt, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		// Since we can't easily mock the SoftKey, let's test the error case instead
		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.Error(err)
		require.Contains(err.Error(), "mock key - check parameters only")
		require.Empty(privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects custom option", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}
		expectedPrivKey := "0xcustomprivatekey123456789"

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which private key do you want to use to test transaction?", expectedOptions).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureString", "Private Key").Return(expectedPrivKey, nil).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.NoError(err)
		require.Equal(expectedPrivKey, privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects genesis key option", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		genesisKeyOpt := "Use the private key of the Genesis Allocated address " + genesisAddress
		customOpt := Custom
		expectedOptions := []string{genesisKeyOpt, cliKeyOpt, customOpt}

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which private key do you want to use to test transaction?", expectedOptions).Return(genesisKeyOpt, nil).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, genesisPrivateKey)
		require.NoError(err)
		require.Equal(genesisPrivateKey, privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureList", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("", errors.New("list error")).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.Error(err)
		require.Contains(err.Error(), "list error")
		require.Empty(privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureString in custom", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		customOpt := Custom
		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureString", "Private Key").Return("", errors.New("string error")).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.Error(err)
		require.Contains(err.Error(), "string error")
		require.Empty(privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from getKey", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, errors.New("getKey error")
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(cliKeyOpt, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.Error(err)
		require.Contains(err.Error(), "getKey error")
		require.Empty(privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureKeyName with invalid keyDir", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		// Use a path that doesn't exist to trigger directory access error
		invalidKeyDir := filepath.Join(os.TempDir(), "nonexistent-key-dir-12345")

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called when CaptureKeyName fails")
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(cliKeyOpt, nil).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, invalidKeyDir, getKey, genesisAddress, "")
		require.Error(err)
		// The error should be related to directory access
		require.True(err.Error() != "")
		require.Empty(privateKey)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects CLI key option - success with real key", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()
		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}

		// Create a real SoftKey for testing
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return realKey, nil
		}

		mockPrompt.On("CaptureList", "Which private key do you want to use to test transaction?", expectedOptions).Return(cliKeyOpt, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		privateKey, err := PromptPrivateKey(mockPrompt, goal, keyDir, getKey, genesisAddress, "")
		require.NoError(err)
		require.NotEmpty(privateKey)
		// Verify that the returned private key matches what we expect from k.PrivKeyHex()
		require.Equal(realKey.PrivKeyHex(), privateKey)
		// Verify it's a valid hex string (should be 64 chars for a 32-byte key)
		require.Equal(64, len(privateKey))
		mockPrompt.AssertExpectations(t)
	})
}

func TestPromptAddress(t *testing.T) {
	require := require.New(t)
	goal := testTransaction
	genesisAddress := "0x1234567890123456789012345678901234567890"
	network := models.NewLocalNetwork()
	customPrompt := "Enter your address"

	t.Run("user selects CLI key option with EVM format", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		cliKeyOpt := cliAddrOpt
		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return nil, errors.New("mock key - check parameters only")
		}

		mockPrompt.On("CaptureList", "Which address do you want to test transaction?", expectedOptions).Return(cliKeyOpt, nil).Once()
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, EVMFormat, customPrompt)
		require.Error(err)
		require.Contains(err.Error(), "mock key - check parameters only")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects custom option with EVM format", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		cliKeyOpt := cliAddrOpt
		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}
		expectedAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which address do you want to test transaction?", expectedOptions).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureAddress", customPrompt).Return(expectedAddr, nil).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, EVMFormat, customPrompt)
		require.NoError(err)
		require.Equal(expectedAddr.Hex(), address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects custom option with PChain format", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		cliKeyOpt := cliAddrOpt
		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}
		expectedAddress := "P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u6wmu4t"

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which address do you want to test transaction?", expectedOptions).Return(customOpt, nil).Once()
		mockPrompt.On("CapturePChainAddress", customPrompt, network).Return(expectedAddress, nil).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, PChainFormat, customPrompt)
		require.NoError(err)
		require.Equal(expectedAddress, address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects custom option with XChain format", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		cliKeyOpt := cliAddrOpt
		customOpt := Custom
		expectedOptions := []string{cliKeyOpt, customOpt}
		expectedAddress := "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u6wmu4t"

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which address do you want to test transaction?", expectedOptions).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureXChainAddress", customPrompt, network).Return(expectedAddress, nil).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, XChainFormat, customPrompt)
		require.NoError(err)
		require.Equal(expectedAddress, address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("user selects genesis key option", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		genesisKeyOpt := "Use the Genesis Allocated address " + genesisAddress
		cliKeyOpt := cliAddrOpt
		customOpt := Custom
		expectedOptions := []string{genesisKeyOpt, cliKeyOpt, customOpt}

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		mockPrompt.On("CaptureList", "Which address do you want to test transaction?", expectedOptions).Return(genesisKeyOpt, nil).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, genesisAddress, network, EVMFormat, customPrompt)
		require.NoError(err)
		require.Equal(genesisAddress, address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureList", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("", errors.New("list error")).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, EVMFormat, customPrompt)
		require.Error(err)
		require.Contains(err.Error(), "list error")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureAddress in custom EVM", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		customOpt := Custom
		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureAddress", customPrompt).Return(common.Address{}, errors.New("address error")).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, EVMFormat, customPrompt)
		require.Error(err)
		require.Contains(err.Error(), "address error")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CapturePChainAddress in custom", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		customOpt := Custom
		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(customOpt, nil).Once()
		mockPrompt.On("CapturePChainAddress", customPrompt, network).Return("", errors.New("pchain error")).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, PChainFormat, customPrompt)
		require.Error(err)
		require.Contains(err.Error(), "pchain error")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureXChainAddress in custom", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()

		customOpt := Custom
		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, nil
		}

		mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(customOpt, nil).Once()
		mockPrompt.On("CaptureXChainAddress", customPrompt, network).Return("", errors.New("invalid network for xchain")).Once()

		address, err := PromptAddress(mockPrompt, goal, keyDir, getKey, "", network, XChainFormat, customPrompt)
		require.Error(err)
		require.Contains(err.Error(), "invalid network for xchain")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})
}

func TestCaptureKeyAddress(t *testing.T) {
	require := require.New(t)
	goal := testTransaction

	t.Run("PChain format with Local network - success", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()

		// Create a real SoftKey for testing
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return realKey, nil
		}

		// For Local network, includeEwoq should be true
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
		require.NoError(err)
		require.NotEmpty(address)
		require.True(len(address) > 0)
		// Verify it's a P-Chain address format
		require.True(len(realKey.P()) > 0)
		require.Equal(realKey.P()[0], address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("XChain format with Local network - success", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()

		// Create a real SoftKey for testing
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return realKey, nil
		}

		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, XChainFormat)
		require.NoError(err)
		require.NotEmpty(address)
		require.True(len(address) > 0)
		// Verify it's an X-Chain address format
		require.True(len(realKey.X()) > 0)
		require.Equal(realKey.X()[0], address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("EVM format with Local network - success", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()

		// Create a real SoftKey for testing
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewLocalNetwork(), network)
			require.False(loadStakingSignerKey)
			return realKey, nil
		}

		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, EVMFormat)
		require.NoError(err)
		require.NotEmpty(address)
		require.True(len(address) > 0)
		// Verify it's an EVM address format (starts with 0x and is 42 chars)
		require.True(len(address) == 42)
		require.True(address[:2] == "0x")
		require.Equal(realKey.C(), address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("PChain format with Fuji network - success", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewFujiNetwork()

		// Create a real SoftKey for testing with Fuji network ID
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(keyName string, network models.Network, loadStakingSignerKey bool) (*key.SoftKey, error) {
			require.Equal("testkey", keyName)
			require.Equal(models.NewFujiNetwork(), network)
			require.False(loadStakingSignerKey)
			return realKey, nil
		}

		// For Fuji network, includeEwoq should be false
		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
		require.NoError(err)
		require.NotEmpty(address)
		require.True(len(realKey.P()) > 0)
		require.Equal(realKey.P()[0], address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("error from CaptureKeyName", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir() // Empty directory

		// Use Fuji network so includeEwoq=false, ensuring no default keys are available
		network := models.NewFujiNetwork()

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			t.Errorf("getKey should not be called")
			return nil, nil
		}

		// With no keys in directory and includeEwoq=false, should get "no keys" error
		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
		require.Error(err)
		require.Contains(err.Error(), "no keys")
		require.Empty(address)
	})

	t.Run("error from getKey", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return nil, errors.New("getKey failed")
		}

		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
		require.Error(err)
		require.Contains(err.Error(), "getKey failed")
		require.Empty(address)
		mockPrompt.AssertExpectations(t)
	})

	t.Run("undefined format returns empty string", func(*testing.T) {
		mockPrompt := &mocks.Prompter{}
		keyDir := t.TempDir()
		keyFile := filepath.Join(keyDir, "testkey.pk")
		err := os.WriteFile(keyFile, []byte("dummy"), 0o600)
		require.NoError(err)

		network := models.NewLocalNetwork()

		// Create a real SoftKey for testing
		realKey, err := key.NewSoft(network.ID)
		require.NoError(err)

		getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
			return realKey, nil
		}

		mockPrompt.On("CaptureListWithSize", mock.Anything, mock.Anything, mock.Anything).Return("testkey", nil).Once()

		address, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, Undefined)
		require.NoError(err)
		require.Empty(address) // Undefined format should return empty string
		mockPrompt.AssertExpectations(t)
	})

	t.Run("includeEwoq logic verification", func(*testing.T) {
		// Test that the includeEwoq parameter is correctly set based on network type

		t.Run("Local network sets includeEwoq=true", func(*testing.T) {
			mockPrompt := &mocks.Prompter{}
			keyDir := t.TempDir()

			network := models.NewLocalNetwork()

			getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
				return nil, errors.New("testing includeEwoq logic")
			}

			// With includeEwoq=true and empty dir, "ewoq" should be available as default key,
			// so CaptureListWithSize should be called
			mockPrompt.On("CaptureListWithSize", mock.Anything, []string{"ewoq"}, 1).Return("ewoq", nil).Once()

			_, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
			require.Error(err)
			require.Contains(err.Error(), "testing includeEwoq logic")
			mockPrompt.AssertExpectations(t)
		})

		t.Run("Fuji network sets includeEwoq=false", func(*testing.T) {
			mockPrompt := &mocks.Prompter{}
			keyDir := t.TempDir()

			network := models.NewFujiNetwork()

			getKey := func(string, models.Network, bool) (*key.SoftKey, error) {
				return nil, errors.New("testing includeEwoq logic for Fuji")
			}

			// With includeEwoq=false and empty dir, should get no keys error
			_, err := CaptureKeyAddress(mockPrompt, goal, keyDir, getKey, network, PChainFormat)
			require.Error(err)
			require.Contains(err.Error(), "no keys")
		})
	})
}
