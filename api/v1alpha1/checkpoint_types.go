/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:validation:Required
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type credentials struct {
}

type objectStorage struct {
	// +required
	// +kubebuilder:validation:MinLength:=1
	Bucket string `json:"bucket"`
	// +optional
	Prefix string `json:"prefix"`
	// +optional
	Config map[string]string `json:"config"`
	// +required
	Credentials credentials `json:"credentials"`
}

type s3Storage struct {
	bucket      string
	prefix      string
	config      map[string]string
	Credentials credentials
}
type azureBlobStorage struct {
	bucket      string
	prefix      string
	config      map[string]string
	Credentials credentials
}
type googleCloudStorage struct {
	bucket      string
	prefix      string
	config      map[string]string
	Credentials credentials
}

type objectStorages struct {
	// +optional
	Aws []objectStorage `json:"aws,omitempty"`
	// +optional
	Azure []objectStorage `json:"azure,omitempty"`
	// +optional
	Google []objectStorage `json:"google,omitempty"`
}

type CheckPointSpec struct {
	// +required
	// +kubebuilder:validation:MinLength:=1
	PodName string `json:"podname"`
	// +optional
	ContainerName string `json:"containerName,omitempty"`
	// +required
	// +default:value="default"
	NameSpace string `json:"namespace,omitempty"`
	// +required
	// +default:value="gzip"
	// +kubebuilder:validation:Enum:=gzip;bzip2;xz;lzip;zstd
	Compression string `json:"compression,omitempty"`
	// +optional
	Path string `json:"path,omitempty"`
	// +optional
	Storage objectStorages `json:"storage,omitempty"`
}

type CheckPointStatus struct {
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CheckPoint is the Schema for the checkpoints API
type CheckPoint struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CheckPoint
	// +required
	Spec CheckPointSpec `json:"spec"`

	// status defines the observed state of CheckPoint
	// +optional
	Status CheckPointStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CheckPointList contains a list of CheckPoint
type CheckPointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CheckPoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheckPoint{}, &CheckPointList{})
}
