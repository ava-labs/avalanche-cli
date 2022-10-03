package plugins

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetVMID_imported(t *testing.T) {
	assert := assert.New(t)
	testVMID := "abcd"
	sc := models.Sidecar{
		ImportedFromAPM: true,
		ImportedVMID:    testVMID,
	}

	vmid, err := getVMID(sc)
	assert.NoError(err)
	assert.Equal(testVMID, vmid)
}

func TestGetVMID_derived(t *testing.T) {
	assert := assert.New(t)
	testVMName := "subnet"
	sc := models.Sidecar{
		ImportedFromAPM: false,
		Name:            testVMName,
	}

	expectedVMID, err := utils.VMID(testVMName)
	assert.NoError(err)

	vmid, err := getVMID(sc)
	assert.NoError(err)
	assert.Equal(expectedVMID.String(), vmid)
}

// func TestManualUpgrade(t *testing.T) {
// 	subnetName := "subnet"
// 	vmName := models.SubnetEvm
// 	targetVersion := "v1.2.0"

// 	assert := assert.New(t)
// 	testDir := t.TempDir()
// 	logFile := filepath.Join(testDir, "prints")

// 	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, constants.DefaultPerms755)
// 	assert.NoError(err)
// 	defer f.Close()

// 	ux.NewUserLog(logging.NoLog{}, f)

// 	app := &application.Avalanche{}
// 	mockPrompt := mocks.NewPrompter(t)
// 	app.Setup(testDir, logging.NoLog{}, config.New(), mockPrompt)

// 	sc := models.Sidecar{
// 		Name:      subnetName,
// 		VM:        models.VMType(vmName),
// 		VMVersion: "v1.0.0",
// 	}

// 	err = ManualUpgrade(app, sc, targetVersion)
// 	assert.NoError(err)

// 	err = f.Close()
// 	assert.NoError(err)

// 	// read output file
// 	output, err := os.ReadFile(logFile)
// 	assert.NoError(err)

// 	assert.Contains(string(output), "1. Replace your VM binary in your node's plugin directory")
// }
