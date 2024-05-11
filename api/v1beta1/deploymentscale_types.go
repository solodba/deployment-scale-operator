/*
Copyright 2024.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeploymentScaleSpec defines the desired state of DeploymentScale
type DeploymentScaleSpec struct {
	// 任务是否开启
	Active bool `json:"active"`
	// 任务开始时间
	StartTime string `json:"startTime"`
	// 任务结束时间
	EndTime string `json:"endTime"`
	// 任务周期间隔
	Period int `json:"period"`
	// 扩展数量
	Replicas int32 `json:"replicas"`
	// Deployments
	Deployments []*Deployment `json:"deployments"`
}

// Deployment
type Deployment struct {
	// deployment名称
	Name string `json:"name"`
	// deployment所在命名空间
	Namespace string `json:"namespace"`
}

// DeploymentScaleStatus defines the observed state of DeploymentScale
type DeploymentScaleStatus struct {
	// 任务状态
	Active bool `json:"active"`
	// 任务下次启动时间
	NextTime int64 `json:"nextTime"`
	// 任务记录
	LastResult string `json:"lastResult"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Active",type="boolean",JSONPath=`.status.active`
//+kubebuilder:printcolumn:name="NextTime",type="integer",JSONPath=`.status.nextTime`
//+kubebuilder:printcolumn:name="LastResult",type="string",JSONPath=`.status.lastResult`

// DeploymentScale is the Schema for the deploymentscales API
type DeploymentScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentScaleSpec   `json:"spec,omitempty"`
	Status DeploymentScaleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeploymentScaleList contains a list of DeploymentScale
type DeploymentScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentScale `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentScale{}, &DeploymentScaleList{})
}
