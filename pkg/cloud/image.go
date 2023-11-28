// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cloud

import "time"

// Image represents the images on the cloud providers
type Image struct {
	ID        string    `json:"Id"`
	Name      string    `json:"Name"`
	Created   time.Time `json:"Created"`
	CloudType CloudType `json:"CloudType"`
	Region    string    `json:"Region"`
	Tags      Tags      `json:"Tags"`
}

// GetName returns the name of the image
func (img Image) GetName() string {
	return img.Name
}

// GetOwner returns the owner of the image
func (img Image) GetOwner() string {
	return ""
}

// GetCloudType returns the type of the cloud
func (img Image) GetCloudType() CloudType {
	return img.CloudType
}

// GetCreated returns the creation time of the image
func (img Image) GetCreated() time.Time {
	return img.Created
}

// GetItem returns the image struct itself
func (img Image) GetItem() interface{} {
	return img
}

// GetType returns the image's string representation
func (img Image) GetType() string {
	return "image"
}

func (img Image) GetTags() Tags {
	return img.Tags
}
