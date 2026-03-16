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

// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Pending;InProgress;Ready;Failed
type ContainerCheckpointPhase string

const (
	ContainerCheckpointPending    ContainerCheckpointPhase = "Pending"
	ContainerCheckPointInProgress ContainerCheckpointPhase = "InProgress"
	ContainerCheckpointReady      ContainerCheckpointPhase = "Ready"
	ContainerCheckpointFailed     ContainerCheckpointPhase = "Failed"
)

const (
	ConditionReady = "Ready"
)

type credentials struct {
}

// +kubebuilder:object:generate=true
type objectStorage struct {
	// +required
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`
	// +optional
	Prefix string `json:"prefix"`
	// +optional
	Config map[string]string `json:"config"`
	// +required
	Credentials credentials `json:"credentials"`
}

// +kubebuilder:object:generate=true
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
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self==oldSelf",message="podName is immutable"
	PodName string `json:"podName"`
	// +optional
	// +kubebuilder:validation:XValidation:rule="self==oldSelf",message="ContainerName is immutable"
	ContainerName string `json:"containerName,omitempty"`
	// +required
	// +kubebuilder:validation:XValidation:rule="self==oldSelf",message="Namespace is immutable"
	// +default:value="default"
	NameSpace string `json:"namespace,omitempty"`
	// +required
	// +default:value="gzip"
	// +kubebuilder:validation:Enum=gzip;zstd
	Compression string `json:"compression,omitempty"`
	// +optional
	Path string `json:"path,omitempty"`
	// +optional
	// Storage objectStorages `json:"storage,omitempty"`
}

type CheckPointStatus struct {
	// +optional
	Phase ContainerCheckpointPhase `json:"phase,omitempty"`
	// +optional
	CheckPointName string `json:"checkpointname,omitempty"`
	// +optional
	NodeName string `json:"nodeName,omitempty"`
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
