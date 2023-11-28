// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cloud

import "time"

// Disk represents the root and attached disks for the instances
type Disk struct {
	ID        string            `json:"Id"`
	Name      string            `json:"Name"`
	Created   time.Time         `json:"Created"`
	State     State             `json:"State"`
	Owner     string            `json:"Owner"`
	CloudType CloudType         `json:"CloudType"`
	Region    string            `json:"Region"`
	Size      int64             `json:"Size"`
	Type      string            `json:"Type"`
	Metadata  map[string]string `json:"Metadata"`
	Tags      Tags              `json:"Tags"`
}

// GetName returns the name of the disk
func (d Disk) GetName() string {
	return d.Name
}

// GetOwner returns the owner of the disk
func (d Disk) GetOwner() string {
	return d.Owner
}

// GetCloudType returns the type of the cloud
func (d Disk) GetCloudType() CloudType {
	return d.CloudType
}

// GetCreated returns the creation time of the disk
func (d Disk) GetCreated() time.Time {
	return d.Created
}

// GetItem returns the disk struct itself
func (d Disk) GetItem() interface{} {
	return d
}

// GetType returns the disk's string representation
func (d Disk) GetType() string {
	return "disk"
}

func (d Disk) GetTags() Tags {
	return d.Tags
}
