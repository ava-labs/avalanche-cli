// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package capturetests

import (
	"errors"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

/*
	These tests need to be in an independent module
	because otherwise they create a circular dependency
	between the mocks and the prompts modules
*/

func TestListDecision(t *testing.T) {
	assert := assert.New(t)
	mockPrompt := &mocks.Prompter{}

	p := prompts.NewPrompter()

	pk, err := crypto.GenerateKey()
	assert.NoError(err)
	addr := crypto.PubkeyToAddress(pk.PublicKey)
	pk2, err := crypto.GenerateKey()
	assert.NoError(err)
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
	arg := "doesn't matter"

	// 1.cancel
	list, cancel, err := p.CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
		arg,
	)
	assert.NoError(err)
	assert.True(cancel)
	assert.Empty(list)

	// 2. error
	list, cancel, err = p.CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
		arg,
	)
	assert.Error(err)
	assert.ErrorContains(err, "fake error")
	assert.False(cancel)
	assert.Empty(list)

	// 3. add - 1 valid, 1 done
	list, cancel, err = p.CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
		arg,
	)
	assert.NoError(err)
	assert.False(cancel)
	assert.Exactly(1, len(list))

	// 4. add - 1 valid, then add the same
	list, cancel, err = p.CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
		arg,
	)
	assert.NoError(err)
	assert.False(cancel)
	assert.Exactly(2, len(list))

	// 5. add - 2 valid, then remove index 1, readd
	list, cancel, err = p.CaptureListDecision(
		mockPrompt,
		prompt,
		capture,
		capturePrompt,
		label,
		info,
		arg,
	)
	assert.NoError(err)
	assert.False(cancel)
	assert.Exactly(2, len(list))
}
