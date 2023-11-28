// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cloud

// CloudType type of the cloud
type CloudType string

const (
	// AWS cloud type
	AWS = CloudType("AWS")
	// GCP cloud type
	GCP = CloudType("GCP")
	// AZURE cloud type
	AZURE = CloudType("AZURE")
	// AZURE cloud type
	ALIBABA = CloudType("ALIBABA")
	// DUMMY cloud type
	DUMMY = CloudType("DUMMY")
)

// CloudProvider interface for the functions that can be used as operations/actions on the cloud providers
type CloudProvider interface {
	GetCloudType() CloudType
	GetAccountName() string
	GetInstances() ([]*Instance, error)
	StopInstances([]*Instance) []error
	TerminateInstances([]*Instance) []error
}
