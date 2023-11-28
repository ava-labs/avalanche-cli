// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cloud

import "time"

// Instance is a general cloud instance struct processed by filters and actions
type Instance struct {
	ID           string            `json:"Id"`
	Name         string            `json:"Name"`
	Created      time.Time         `json:"Created"`
	Tags         Tags              `json:"Tags"`
	Owner        string            `json:"Owner"`
	CloudType    CloudType         `json:"CloudType"`
	InstanceType string            `json:"InstanceType"`
	State        State             `json:"State"`
	Metadata     map[string]string `json:"Metadata"`
	Region       string            `json:"Region"`
	Ephemeral    bool              `json:"Ephemeral"`
}

// Tags Key-value pairs of the tags on the instances
type Tags map[string]string

// GetName returns the name of the instance
func (i Instance) GetName() string {
	return i.Name
}

// GetOwner returns the 'Owner' tag's value of the instance. If there is not tag present then returns '???'
func (i Instance) GetOwner() string {
	if len(i.Owner) == 0 {
		return "???"
	}
	return i.Owner
}

// GetCloudType returns the type of the cloud (AWS/AZURE/GCP)
func (i Instance) GetCloudType() CloudType {
	return i.CloudType
}

// GetCreated returns the creation time of the instance
func (i Instance) GetCreated() time.Time {
	return i.Created
}

// GetItem returns the cloud instance object itself
func (i Instance) GetItem() interface{} {
	return i
}

// GetType returns the type representation of the instance
func (i Instance) GetType() string {
	return "instance"
}

func (i Instance) GetTags() Tags {
	return i.Tags
}
