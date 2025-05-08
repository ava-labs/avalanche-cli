// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package capturetests

import (
	"errors"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

/*
	These tests need to be in an independent module
	because otherwise they create a circular dependency
	between the mocks and the prompts modules
*/

func TestListDecision(t *testing.T) {
	require := require.New(t)
	mockPrompt := &mocks.Prompter{}

	pk, err := crypto.GenerateKey()
	require.NoError(err)
	addr := crypto.PubkeyToAddress(pk.PublicKey)
	pk2, err := crypto.GenerateKey()
	require.NoError(err)
	addr2 := crypto.PubkeyToAddress(pk2.PublicKey)

	// 1. cancel
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Cancel, nil).Once()

	// 2. error
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("", errors.New("fake error")).Once()

	// 3. add - 1 valid, 1 done
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Done, nil).Once()

	// 4. add - 1 valid, then add the same
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	// do a preview (nothing changes, but prints 2 addrs on STDOUT)
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Preview, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Done, nil).Once()

	// 5. add - 2 valid, then remove index 1, readd
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	// print info (nothing changes, but prints info to STDOUT)
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.MoreInfo, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Del, nil).Once()
	mockPrompt.On("CaptureIndex", mock.Anything, mock.Anything).Return(1, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Add, nil).Once()
	mockPrompt.On("CaptureAddress", mock.Anything, mock.Anything).Return(addr2, nil).Once()
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return(prompts.Done, nil).Once()

	prompt := "Test CaptureListDecision"
	capture := mockPrompt.CaptureAddress
	capturePrompt := "Enter address"
	label := "Test"
	info := "something"

	// 1.cancel
	list, cancel, err := prompts.CaptureListDecision(
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

	// 2. error
	list, cancel, err = prompts.CaptureListDecision(
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

	// 3. add - 1 valid, 1 done
	list, cancel, err = prompts.CaptureListDecision(
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

	// 4. add - 1 valid, then add the same
	list, cancel, err = prompts.CaptureListDecision(
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

	// 5. add - 2 valid, then remove index 1, readd
	list, cancel, err = prompts.CaptureListDecision(
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
}
