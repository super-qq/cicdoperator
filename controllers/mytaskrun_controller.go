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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientsetCore "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cicdoperatorv1 "qi1999.io/cicdoperator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	WorkSpacePrefix       = "/internal/workspaces"
	ResultPrefix          = "/internal/results"
	ScriptPrefix          = "/internal/scripts"
	StepPrefix            = "/internal/steps"
	InternalWorkspaceName = "internal-workspaces"
	InternalResultName    = "internal-results"
	InternalScripName     = "internal-scripts"
	InternalStepName      = "internal-steps"
)

// 内部的emptydir volume

var (
	internalVolumes = []corev1.Volume{{
		Name:         InternalResultName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         InternalStepName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}
	internalVolumeMounts = []corev1.VolumeMount{{
		Name:      InternalResultName,
		MountPath: ResultPrefix,
	}, {
		Name:      InternalStepName,
		MountPath: StepPrefix,
		ReadOnly:  true,
	},
	}
)

// MyTaskRunReconciler reconciles a MyTaskRun object
type MyTaskRunReconciler struct {
	CoreClientSet *clientsetCore.Clientset // 操作core 对象的client
	client.Client
	Scheme *runtime.Scheme
}

// 校验taskRun的参数
func validateParams(params []cicdoperatorv1.ParamSpec, paramRuns []cicdoperatorv1.ParamRun, actualVarMap map[string]string) error {
	// 必须提供的参数
	mustParamsMap := make(map[string]string)
	// 选填的参数，有默认值
	defaultParamsMap := make(map[string]string)

	// 遍历task中的params，把
	for _, p := range params {
		p := p
		if p.HasDefault {
			defaultParamsMap[fmt.Sprintf("params.%s", p.Name)] = p.Default
			continue
		}
		mustParamsMap[fmt.Sprintf("params.%s", p.Name)] = ""
	}

	// taskRun的参数
	for _, p := range paramRuns {
		p := p
		actualVarMap[fmt.Sprintf("params.%s", p.Name)] = p.Value
	}

	// 校验
	for k := range mustParamsMap {
		if _, ok := actualVarMap[k]; !ok {
			return fmt.Errorf("missing values for these params which have no default values: %s", k)
		}
	}
	for k, v := range defaultParamsMap {
		if _, ok := actualVarMap[k]; !ok {
			// 说明run没有提供default ,那么将default塞进去
			actualVarMap[k] = v
		}
	}

	return nil
}

// 校验workspace
func validateWorkspaces(decl []cicdoperatorv1.WorkspaceDeclaration, actual []cicdoperatorv1.WorkspaceBinding, actualVarMap map[string]string) error {

	declMap := map[string]string{}
	actualMap := map[string]string{}
	for i := range decl {
		declMap[fmt.Sprintf("workspaces.%s", decl[i].Name)] = fmt.Sprintf("%s/%s", WorkSpacePrefix, decl[i].Name)
	}
	for i := range actual {
		actualMap[fmt.Sprintf("workspaces.%s", actual[i].Name)] = fmt.Sprintf("%s/%s", WorkSpacePrefix, actual[i].Name)
	}
	for k, v := range declMap {
		if _, ok := actualMap[k]; !ok {
			return fmt.Errorf("missing values for these workspace which have no provide: %s", k)
		}
		actualVarMap[k] = v

	}
	return nil

}

//+kubebuilder:rbac:groups=cicdoperator.qi1999.io,resources=mytaskruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cicdoperator.qi1999.io,resources=mytaskruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cicdoperator.qi1999.io,resources=mytaskruns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MyTaskRun object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MyTaskRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// 获取这个LogBackend crd ,这里是检查这个 crd资源是否存在
	instance := &cicdoperatorv1.MyTaskRun{}
	actualVarMap := map[string]string{}

	klog.Infof("[Reconcile call  start][ns:%v][MyTaskRun:%v]", req.Namespace, req.Name)
	// 唯一的标识 ，这里用namespace+name 是为了防止同名对象出现在多个ns中
	//uniqueName := req.NamespacedName.String()
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Errorf("[ Reconcile start missing be deleted][ns:%v][MyTaskRun:%v]", req.Namespace, req.Name)
			// 如果错误是不存在，那么可能是到调谐这里 就被删了
			return reconcile.Result{}, nil
		}
		// 其它错误打印一下
		klog.Errorf("[ Reconcile start other error][err:%v][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
		return reconcile.Result{}, err
	}
	// 从这里开始判断，对象是新的还是旧的
	// 如果status中的TaskSpec指针为空，说明还没有被控制器管理，属于新对象
	if instance.Status.TaskSpec == nil {
		klog.Infof("[MyTaskRun.new.try.find.Task][ns:%v][MyTaskRun:%v]", req.Namespace, req.Name)
		// 根据配置的taskRef 找到task模板信息
		// var taskObj *cicdoperatorv1.MyTask
		taskObj := &cicdoperatorv1.MyTask{}
		taskRefNamespacedName := types.NamespacedName{
			Namespace: req.Namespace,
			Name:      instance.Spec.TaskRef,
		}
		err = r.Client.Get(context.TODO(), taskRefNamespacedName, taskObj)
		if err != nil {
			if errors.IsNotFound(err) {
				err := fmt.Errorf("taskOjb nil ")
				klog.Errorf("[MyTaskRun.new.add.task.nil.err][err:%v][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
				// 5秒后入队
				return reconcile.Result{RequeueAfter: time.Second * 5}, nil
			}
			err := fmt.Errorf("get taskOjb err ")
			klog.Errorf("[MyTaskRun.new.add.task.other.err][err:%v][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
			// 5秒后入队
			return reconcile.Result{}, err
		}
		klog.Infof("[MyTaskRun.new.succ.find.Task][ns:%v][taskObj:%+v]", req.Namespace, taskObj)

		// 校验传入的参数
		err = validateParams(taskObj.Spec.Params, instance.Spec.Params, actualVarMap)
		if err != nil {
			klog.Errorf("[MyTaskRun.new.add.task.validateParams.err][err:%v][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
			return reconcile.Result{}, err
		}
		// 校验workspace
		err = validateWorkspaces(taskObj.Spec.Workspaces, instance.Spec.Workspaces, actualVarMap)
		if err != nil {
			klog.Errorf("[MyTaskRun.new.add.task.validateWorkspaces.err][err:%v][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
			return reconcile.Result{}, err
		}

		// 设置result 的path
		for i := range taskObj.Spec.Results {
			actualVarMap[fmt.Sprintf("results.%s", taskObj.Spec.Results[i].Name)] = fmt.Sprintf("%s/%s", ResultPrefix, taskObj.Spec.Results[i].Name)
		}

		// 设置TaskSpec
		instance.Status.TaskSpec = &taskObj.Spec
		err = r.Status().Update(ctx, instance)
		if err != nil {
			klog.Errorf("[MyTaskRun.new.updateStatus.err][ns:%v][MyTaskRun:%v][err:%v]", req.Namespace, req.Name, err)
			return reconcile.Result{}, err
		}
		klog.Infof("[MyTaskRun.new.Status.TaskSpec.set.success][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)

	}

	// 这里要获取最新的pod 指针对象，
	var pod *corev1.Pod

	if instance.Status.PodName != "" {
		// 如果podName存在 就获取
		pod, err = r.CoreClientSet.CoreV1().Pods(instance.Namespace).Get(ctx, instance.Status.PodName, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			// 如果还没找到，可能还在创建
			// Keep going, this will result in the Pod being created below.
		} else if err != nil {
			// This is considered a transient error, so we return error, do not update
			// the task run condition, and return an error which will cause this key to
			// be requeued for reconcile.
			klog.Errorf("Error getting pod %q: %v", instance.Status.PodName, err)
			return reconcile.Result{}, err
		}
	} else {
		// 这里看看要不要收留孤儿pod
	}

	// 走到这里说明pod确实还没创建，创建它
	if pod == nil {
		if len(actualVarMap) == 0 {
			// 说明是设置taskspec的status update 触发的reconcile
			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
		}
		pod, err = r.createPod(ctx, instance, actualVarMap)
		if err != nil {
			klog.Errorf("Failed to create task run pod for taskrun %q: %v", instance.Name, err)
			return reconcile.Result{}, err
		}
		instance.Status.PodName = pod.Name
		// 这里已经开始，需要将状态设置为开始
		instance.Status.CurrentStatus = cicdoperatorv1.TaskRunReasonStarted
		instance.Status.StartTime = &metav1.Time{Time: time.Now()}
		err = r.Status().Update(ctx, instance)
		//err = r.Update(ctx, instance)
		if err != nil {
			klog.Errorf("[MyTaskRun.createPod.updateStatus.err][ns:%v][MyTaskRun:%v][err:%v]", req.Namespace, req.Name, err)
			return reconcile.Result{}, err
		}
		klog.Infof("[MyTaskRun.createPod.TaskSpec.set.success][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)
	}
	// 这里需要判断pod的状态
	// 这里得判断一下状态是否发生变化
	lastStatus := instance.Status.CurrentStatus
	lastStep := instance.Status.CurrentStep
	statusChanged := false
	runEnd := false
	if string(pod.Status.Phase) != lastStatus {
		statusChanged = true
	}
	switch pod.Status.Phase {
	// pending的处理 init的状态是pending ,这里也把它算running
	case corev1.PodRunning, corev1.PodPending:
		// running 的不需要再做什么，返回等下一轮调谐即可
		instance.Status.CurrentStatus = cicdoperatorv1.TaskRunReasonRunning
	case corev1.PodSucceeded:
		runEnd = true
		instance.Status.CurrentStatus = cicdoperatorv1.TaskRunReasonSuccessful
		instance.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	case corev1.PodFailed:
		runEnd = true
		instance.Status.CurrentStatus = cicdoperatorv1.TaskRunReasonFailed
		instance.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	default:

	}
	// 遍历pod的 initContainerStatus 和containerStatus设置step status
	stepStatus := make([]cicdoperatorv1.StepState, 0)
	// 根据容器的.State.Terminated != nil 判断当前执行到哪个了
	stepIndex := 0
	for i := range pod.Status.InitContainerStatuses {
		ps := pod.Status.InitContainerStatuses[i]
		if ps.State.Terminated != nil {
			stepIndex++
		}

		stepStatus = append(stepStatus, cicdoperatorv1.StepState{
			ContainerState: ps.State,
			Name:           ps.Name,
			ContainerName:  ps.Name,
			ImageID:        ps.ImageID,
		})
	}
	for i := range pod.Status.ContainerStatuses {
		ps := pod.Status.ContainerStatuses[i]
		if ps.State.Terminated != nil {
			stepIndex++
		}
		stepStatus = append(stepStatus, cicdoperatorv1.StepState{
			ContainerState: ps.State,
			Name:           ps.Name,
			ContainerName:  ps.Name,
			ImageID:        ps.ImageID,
		})
	}
	instance.Status.Steps = stepStatus
	// 根据step索引设置当前的状态
	allStepNum := len(instance.Status.TaskSpec.Steps)
	if allStepNum == 1 {
		stepIndex = 0
	}
	if stepIndex == allStepNum {
		stepIndex = allStepNum - 1
	}
	thisStepName := instance.Status.TaskSpec.Steps[stepIndex].Name
	if thisStepName != lastStep {
		statusChanged = true
	}

	klog.Infof("[MyTaskRun.reconcilePod.detail.print][tr:%v][string(pod.Status.Phase):%v][lastStatus:%v][thisStatus:%v][lastStep:%v][thisStep:%v][statusChanged:%v][runEnd:%v]",
		instance.Name,
		string(pod.Status.Phase),
		lastStatus,
		instance.Status.CurrentStatus,
		lastStep,
		thisStepName,
		statusChanged,
		runEnd,
	)

	instance.Status.CurrentStep = thisStepName
	err = r.Status().Update(ctx, instance)
	//err = r.Update(ctx, instance)
	if err != nil {
		klog.Errorf("[MyTaskRun.reconcilePod.updateStatus.err][ns:%v][MyTaskRun:%v][err:%v]", req.Namespace, req.Name, err)
		return reconcile.Result{}, err
	}
	klog.Infof("[MyTaskRun.reconcilePod.updateStatus.success][ns:%v][MyTaskRun:%v]", req.Namespace, req.Name)

	if !runEnd {

		// 如果状态没变化，那么再重新入队再走一遍
		// 相当于根据podstatus 做watchpod
		return reconcile.Result{RequeueAfter: time.Second * 2}, nil
	}
	//if statusChanged {
	//
	//}
	return reconcile.Result{}, nil
}

func (r *MyTaskRunReconciler) replaceVariables(spec *cicdoperatorv1.MyTaskSpec, actualVarMap map[string]string) *cicdoperatorv1.MyTaskSpec {
	spec = spec.DeepCopy()

	for i := range spec.Steps {
		r.replaceStep(&spec.Steps[i], actualVarMap)
	}
	return spec

}

func (r *MyTaskRunReconciler) replaceStep(step *cicdoperatorv1.Step, actualMap map[string]string) {
	// 替换script
	step.Script = ApplyReplacements(step.Script, actualMap)
	// 替换workingDir
	step.WorkingDir = ApplyReplacements(step.WorkingDir, actualMap)
	// 替换Image
	step.Image = ApplyReplacements(step.Image, actualMap)
	// 替换args
	for i := range step.Args {
		step.Args[i] = ApplyReplacements(step.Args[i], actualMap)
	}

	// 替换command
	for i := range step.Command {
		step.Command[i] = ApplyReplacements(step.Command[i], actualMap)
	}

}

// in 代表输入的字符串
// replacements 代表变量 名到value的 map
func ApplyReplacements(in string, replacements map[string]string) string {
	replacementsList := []string{}
	for k, v := range replacements {
		replacementsList = append(replacementsList, fmt.Sprintf("$(%s)", k), v)
	}
	// strings.Replacer does all replacements in one pass, preventing multiple replacements
	// See #2093 for an explanation on why we need to do this.
	replacer := strings.NewReplacer(replacementsList...)
	return replacer.Replace(in)
}

func (r *MyTaskRunReconciler) createPod(ctx context.Context, tr *cicdoperatorv1.MyTaskRun, actualVarMap map[string]string) (*corev1.Pod, error) {
	klog.Infof("[myTaskRun.createPod.start][tr:%v]", tr.Name)
	time.Sleep(2 * time.Second)
	tr.Status.TaskSpec = r.replaceVariables(tr.Status.TaskSpec, actualVarMap)
	// 生成内部的volume
	volumes := make([]corev1.Volume, 0)
	volumes = append(volumes, internalVolumes...)
	// 生成公共的volumeMount
	commonVolumeMount := make([]corev1.VolumeMount, 0)
	commonVolumeMount = append(commonVolumeMount, internalVolumeMounts...)
	// 解析workSpace产生的volume和volumeMounts
	wsVs, wsVms := createWorkspaceVolumesAndMount(tr.Spec.Workspaces)
	volumes = append(volumes, wsVs...)
	commonVolumeMount = append(commonVolumeMount, wsVms...)

	initContainers := make([]corev1.Container, 0)
	mainContainers := make([]corev1.Container, 0)
	stepNum := len(tr.Status.TaskSpec.Steps)
	for i := range tr.Status.TaskSpec.Steps {
		step := tr.Status.TaskSpec.Steps[i]
		thisC := corev1.Container{}
		thisC.Name = step.Name
		thisC.Args = step.Args
		thisC.Image = step.Image
		thisC.WorkingDir = step.WorkingDir
		thisC.ImagePullPolicy = step.ImagePullPolicy
		thisVolumeMount := commonVolumeMount
		thisC.VolumeMounts = thisVolumeMount
		if step.Script != "" {
			// 创建configMap
			configMapName := fmt.Sprintf("mytaskrun-%s-step-%s-script", tr.Name, step.Name)

			cmObj, err := r.CoreClientSet.CoreV1().ConfigMaps(tr.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				cmObj = &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapName,
						Namespace: tr.Namespace,
						// 设置这个删除父 ，默认级联删除
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(tr, tr.GroupVersionKind()),
						},
					},
					Data: map[string]string{
						"script": step.Script,
					},
				}
				// 不存在再创建
				_, err := r.CoreClientSet.CoreV1().ConfigMaps(tr.Namespace).Create(ctx, cmObj, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("[myTaskRun.createPod.create.script.configmap.err][tr:%v][cm:%v][err:%v]", tr.Name, configMapName, err)
					return nil, err
				}
				klog.Infof("[myTaskRun.createPod.create.script.configmap.success][tr:%v][cm:%v]", tr.Name, configMapName)
			} else if err != nil {
				return nil, err
			}

			var modNum *int32
			var modNumValue int32 = 0777
			modNum = &modNumValue
			cm := &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				DefaultMode: modNum,
			}
			tmp := corev1.Volume{
				Name: configMapName,

				VolumeSource: corev1.VolumeSource{ConfigMap: cm},
			}
			volumes = append(volumes, tmp)
			scriptMountPath := fmt.Sprintf("%s/%s", ScriptPrefix, configMapName)
			scVolumeMount := corev1.VolumeMount{
				Name:      configMapName,
				MountPath: scriptMountPath,
			}

			thisVolumeMount = append(thisVolumeMount, scVolumeMount)
			newCmd := fmt.Sprintf("ls -l %s/script ;cat %s/script ; bash %s/script",
				scriptMountPath,
				scriptMountPath,
				scriptMountPath,
			)
			//thisC.Command = []string{"/bin/sh", "-c", newCmd}
			thisC.Command = []string{"bash", "-c", newCmd}
			//thisC.Command = []string{newCmd}

		}
		thisC.VolumeMounts = thisVolumeMount
		// 当前最后一个作为main
		if i+1 == stepNum {
			mainContainers = append(mainContainers, thisC)
		} else {
			initContainers = append(initContainers, thisC)
		}

	}

	newPodName := fmt.Sprintf("mytaskrun-%s-pod", tr.Name)
	newPod, err := r.CoreClientSet.CoreV1().Pods(tr.Namespace).Get(ctx, newPodName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		newPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				// We execute the build's pod in the same namespace as where the build was
				// created so that it can access colocated resources.
				Namespace: tr.Namespace,
				// Generate a unique name based on the build's name.
				// The name is univocally generated so that in case of
				// stale informer cache, we never create duplicate Pods
				Name: newPodName,
				// If our parent TaskRun is deleted, then we should be as well.
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(tr, tr.GroupVersionKind()),
				},
				//Annotations: podAnnotations,
				//Labels:      makeLabels(taskRun),
			},
			Spec: corev1.PodSpec{
				RestartPolicy:  corev1.RestartPolicyNever,
				InitContainers: initContainers,
				Containers:     mainContainers,
				Volumes:        volumes,
			},
		}

		newPod, err = r.CoreClientSet.CoreV1().Pods(tr.Namespace).Create(ctx, newPod, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("[myTaskRun.createPod.error][tr:%v]", tr.Name)
			return nil, err
		}

	} else if err != nil {
		return nil, err
	}

	return newPod, nil
}

func createWorkspaceVolumesAndMount(wb []cicdoperatorv1.WorkspaceBinding) (vs []corev1.Volume, vms []corev1.VolumeMount) {

	for _, w := range wb {
		w := w
		vName := fmt.Sprintf("%s-%s", InternalWorkspaceName, w.Name)
		tmpv := corev1.Volume{
			Name: vName,
		}
		tmpvm := corev1.VolumeMount{
			Name:      vName,
			MountPath: fmt.Sprintf("%s/%s", WorkSpacePrefix, w.Name),
			SubPath:   w.SubPath,
		}

		switch {

		case w.PersistentVolumeClaim != nil:

			pvc := *w.PersistentVolumeClaim
			tmpv.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: &pvc}

		case w.EmptyDir != nil:
			ed := *w.EmptyDir
			tmpv.VolumeSource = corev1.VolumeSource{EmptyDir: &ed}
		case w.ConfigMap != nil:
			cm := *w.ConfigMap
			tmpv.VolumeSource = corev1.VolumeSource{ConfigMap: &cm}

		case w.Secret != nil:
			s := *w.Secret
			tmpv.VolumeSource = corev1.VolumeSource{Secret: &s}
		}
		vs = append(vs, tmpv)
		vms = append(vms, tmpvm)
	}
	return vs, vms
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyTaskRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cicdoperatorv1.MyTaskRun{}).
		Complete(r)
}
