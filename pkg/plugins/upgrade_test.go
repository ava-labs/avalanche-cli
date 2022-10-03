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
