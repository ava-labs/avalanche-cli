package subnetcmd

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddValidator(t *testing.T) {
	assert := assert.New(t)
	promptReturnSequence := []string{
		ids.GenerateTestNodeID().String(), // NodeID
		"42",                              // Weight
		time.Now().Add(constants.StakingStartLeeTime + 2).
			Format(constants.TimeParseLayout), // Starttime
		"400h", // Duration
	}
	prompter := &mocks.PromptRunner{}
	promptFunc := func(txt string) prompts.PromptRunner {
		return prompter
	}
	for i := 0; i < len(promptReturnSequence); i++ {
		prompter.On("Run").Return(promptReturnSequence[i], nil).Once()
	}
	prompter.On("SetValidation", mock.Anything).Return(nil)

	selectReturnSequence := []string{
		prompts.Yes,
		models.Fuji.String(),
	}

	selector := &mocks.SelectRunner{}
	selectFunc := func(txt string, i interface{}) prompts.SelectRunner {
		return selector
	}
	for i := 0; i < len(selectReturnSequence); i++ {
		selector.On("Run").Return(0, selectReturnSequence[i], nil).Once()
	}

	app = application.NewTestApp(t)
	sc := &models.Sidecar{
		Name:      "TEST",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
		Subnet:    "TEST",
	}
	app.CreateSidecar(sc)

	args := []string{"TEST"}

	err := addValidator(args, nil, nil, nil)
	assert.ErrorIs(err, errNoSubnetID)

	sc.SubnetID = ids.GenerateTestID()
	app.UpdateSidecar(sc)

	deployer := &mocks.PublicDeployer{}
	deployer.On("AddValidator", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	deployerFunc := func(*application.Avalanche, string, models.Network) subnet.PublicDeployer {
		return deployer
	}
	err = addValidator(args, promptFunc, selectFunc, deployerFunc)
	assert.NoError(err)
	// app is global now, let's be nice and reset to original state...
	app = nil
}
