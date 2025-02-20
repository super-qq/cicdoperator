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

// 参数配置
type ParamSpec struct {
	// Name declares the name by which a parameter is referenced.
	Name string `json:"name"` //名字
	// +optional
	Description string `json:"description,omitempty"` // 描述
	// +optional
	HasDefault bool   `json:"hasDefault"`        // 是否有默认值
	Default    string `json:"default,omitempty"` // 默认值值
}

// 结果 注意这里不需要value，因为task属于 配置
type TaskResult struct {
	// Name the given name
	Name string `json:"name"`

	// Description is a human-readable description of the result
	// +optional
	Description string `json:"description"`
}

// workerspace 代表存储配置
type WorkspaceDeclaration struct {
	Name string `json:"name"`
	// Description is an optional human readable description of this volume.
	// +optional
	Description string `json:"description,omitempty"`
	// +optional
	MountPath string `json:"mountPath,omitempty"` // 挂载点
	ReadOnly  bool   `json:"readOnly,omitempty"`  // 是否只读
	Optional  bool   `json:"optional,omitempty"`  //是否必填，相当于taskRun实例化时 是否必须要填写，就是必须同
}

type Step struct {

	// 名字
	Name string `json:"name"`
	// 镜像名称
	Image string `json:"image,omitempty"`
	// 执行的cmd
	Command []string `json:"command,omitempty"`
	// 参数
	Args []string `json:"args,omitempty"`
	// 容器工作目录
	WorkingDir string `json:"workingDir,omitempty"`
	// 容器端口，复用pod的
	Ports []corev1.ContainerPort `json:"ports,omitempty"`
	// 复用pod的EnvFrom
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// 复用pod的Env
	Env []corev1.EnvVar `json:"env,omitempty"`
	// 复用pod的 资源申请量
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// 复用pod的 VolumeMounts
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// 复用pod的 VolumeDevices
	VolumeDevices []corev1.VolumeDevice `json:"volumeDevices,omitempty"`
	// 复用pod的 LivenessProbe
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// 复用pod的 ReadinessProbe
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// 复用pod的 StartupProbe
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`
	// 复用pod的 Lifecycle
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`
	// 容器终止信息path
	TerminationMessagePath string `json:"terminationMessagePath,omitempty"`
	// 复用pod的 TerminationMessagePolicy
	TerminationMessagePolicy corev1.TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty"`
	// 复用pod的 ImagePullPolicy
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// 复用pod的 SecurityContext
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty" protobuf:"bytes,15,opt,name=securityContext"`

	// 执行脚本
	Script string `json:"script,omitempty"`

	// 执行超时
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MyTaskSpec defines the desired state of MyTask
type MyTaskSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Params     []ParamSpec            `json:"params,omitempty"`     //参数
	Results    []TaskResult           `json:"results,omitempty"`    //结果
	Workspaces []WorkspaceDeclaration `json:"workspaces,omitempty"` //存储
	Steps      []Step                 `json:"steps,omitempty"`      //步骤
}

// MyTaskStatus defines the observed state of MyTask
type MyTaskStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MyTask is the Schema for the mytasks API
type MyTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MyTaskSpec   `json:"spec,omitempty"`
	Status MyTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MyTaskList contains a list of MyTask
type MyTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MyTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MyTask{}, &MyTaskList{})
}
