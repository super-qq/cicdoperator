/*
Copyright 2025.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TaskRunReasonStarted is the reason set when the TaskRun has just started
	TaskRunReasonStarted = "Started"
	// TaskRunReasonRunning is the reason set when the TaskRun is running
	TaskRunReasonRunning = "Running"
	// TaskRunReasonSuccessful is the reason set when the TaskRun completed successfully
	TaskRunReasonSuccessful = "Succeeded"
	// TaskRunReasonFailed is the reason set when the TaskRun completed with a failure
	TaskRunReasonFailed = "Failed"
	// TaskRunReasonCancelled is the reason set when the Taskrun is cancelled by the user
	TaskRunReasonCancelled = "TaskRunCancelled"
	// TaskRunReasonTimedOut is the reason set when the Taskrun has timed out
	TaskRunReasonTimedOut = "TaskRunTimeout"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Param declares an ArrayOrString to use for the parameter called name.
type ParamRun struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// WorkspaceBinding maps a Task's declared workspace to a Volume.
type WorkspaceBinding struct {
	// Name is the name of the workspace populated by the volume.
	Name string `json:"name"`
	// SubPath is optionally a directory on the volume which should be used
	// for this binding (i.e. the volume will be mounted at this sub directory).
	// +optional
	SubPath string `json:"subPath,omsitempty"`
	// VolumeClaimTemplate is a template for a claim that will be created in the same namespace.
	// The PipelineRun controller is responsible for creating a unique claim for each instance of PipelineRun.
	// +optional
	// VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
	// PersistentVolumeClaimVolumeSource represents a reference to a
	// PersistentVolumeClaim in the same namespace. Either this OR EmptyDir can be used.
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// EmptyDir represents a temporary directory that shares a Task's lifetime.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	// Either this OR PersistentVolumeClaim can be used.
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// ConfigMap represents a configMap that should populate this workspace.
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`
	// Secret represents a secret that should populate this workspace.
	// +optional
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
}

// MyTaskRunSpec defines the desired state of MyTaskRun
type MyTaskRunSpec struct {
	Params []ParamRun `json:"params,omitempty"`

	TaskRef  string      `json:"taskRef,omitempty"` // 指定哪个task模板
	TaskSpec *MyTaskSpec `json:"taskSpec,omitempty"`
	// Used for cancelling a taskrun (and maybe more later on)
	// +optional

	Timeout *metav1.Duration `json:"timeout,omitempty"`

	Workspaces []WorkspaceBinding `json:"workspaces,omitempty"`
}

// StepState reports the results of running a step in a Task.
type StepState struct {
	corev1.ContainerState `json:",inline"`
	Name                  string `json:"name,omitempty"`
	ContainerName         string `json:"container,omitempty"`
	ImageID               string `json:"imageID,omitempty"`
}

// TaskRunResult used to describe the results of a task
type TaskRunResult struct {
	// Name the given name
	Name string `json:"name"`

	// Value the given value of the result
	Value string `json:"value"`
}

// 定义status
type MyTaskRunStatus struct {
	// PodName is the name of the pod responsible for executing this task's steps.
	PodName string `json:"podName"` // 创建了哪个pod上

	CurrentStatus string `json:"currentStatus"` // 当前状态
	// StartTime is the time the build is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"` //启动时间

	// CompletionTime is the time the build completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"` // 结束时间

	// Steps describes the state of each build step container.
	// +optional
	// +listType=atomic
	Steps []StepState `json:"steps,omitempty"` // 每个步骤的状态

	// RetriesStatus contains the history of TaskRunStatus in case of a retry in order to keep record of failures.

	TaskRunResults []TaskRunResult `json:"taskResults,omitempty"` // 结果

	TaskSpec *MyTaskSpec `json:"taskSpec,omitempty"` // 指向目标模版的指针
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MyTaskRun is the Schema for the mytaskruns API
type MyTaskRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MyTaskRunSpec   `json:"spec,omitempty"`
	Status MyTaskRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MyTaskRunList contains a list of MyTaskRun
type MyTaskRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MyTaskRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MyTaskRun{}, &MyTaskRunList{})
}
