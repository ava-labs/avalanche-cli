// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// PluginBinaryDownloader is an autogenerated mock type for the PluginBinaryDownloader type
type PluginBinaryDownloader struct {
	mock.Mock
}

// InstallVM provides a mock function with given fields: vmID, vmBin
func (_m *PluginBinaryDownloader) InstallVM(vmID string, vmBin string) error {
	ret := _m.Called(vmID, vmBin)

	if len(ret) == 0 {
		panic("no return value specified for InstallVM")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(vmID, vmBin)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveVM provides a mock function with given fields: vmID
func (_m *PluginBinaryDownloader) RemoveVM(vmID string) error {
	ret := _m.Called(vmID)

	if len(ret) == 0 {
		panic("no return value specified for RemoveVM")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(vmID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpgradeVM provides a mock function with given fields: vmID, vmBin
func (_m *PluginBinaryDownloader) UpgradeVM(vmID string, vmBin string) error {
	ret := _m.Called(vmID, vmBin)

	if len(ret) == 0 {
		panic("no return value specified for UpgradeVM")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(vmID, vmBin)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewPluginBinaryDownloader creates a new instance of PluginBinaryDownloader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPluginBinaryDownloader(t interface {
	mock.TestingT
	Cleanup(func())
}) *PluginBinaryDownloader {
	mock := &PluginBinaryDownloader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
