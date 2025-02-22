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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientsetCore "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cicdoperatorv1 "qi1999.io/cicdoperator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

		// 设置TaskSpec
		instance.Status.TaskSpec = &taskObj.Spec
		err = r.Status().Update(ctx, instance)
		if err != nil {
			klog.Errorf("[MyTaskRun.new.updateStatus.err][ns:%v][MyTaskRun:%v][err:%v]", req.Namespace, req.Name, err)
			return reconcile.Result{}, err
		}
		klog.Infof("[MyTaskRun.new.Status.TaskSpec.set.success][ns:%v][MyTaskRun:%v]", err, req.Namespace, req.Name)

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyTaskRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cicdoperatorv1.MyTaskRun{}).
		Complete(r)
}
