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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorcodehorsecomv1beta1 "github.com/solodba/deployment-scale-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
)

// DeploymentScaleReconciler reconciles a DeploymentScale object
type DeploymentScaleReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	DeploymentScaleQueue map[string]*operatorcodehorsecomv1beta1.DeploymentScale
	Lock                 sync.RWMutex
	Tickers              []*time.Ticker
	Wg                   sync.WaitGroup
	DeploymentK8sMap     map[string]*appsv1.Deployment
}

//+kubebuilder:rbac:groups=operator.codehorse.com,resources=deploymentscales,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.codehorse.com,resources=deploymentscales/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.codehorse.com,resources=deploymentscales/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DeploymentScale object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *DeploymentScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	deploymentScaleK8s := operatorcodehorsecomv1beta1.NewDeploymentScale()
	err := r.Client.Get(ctx, req.NamespacedName, deploymentScaleK8s)
	if err != nil {
		if errors.IsNotFound(err) {
			// 从任务队列中删除
			operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务不存在!", req.NamespacedName.Name)
			if !r.IsStopTask(deploymentScaleK8s.Spec.EndTime) {
				r.DeleteQueue(deploymentScaleK8s)
			} else {
				r.DeploymentShrink()
			}
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, err
	}
	// 对比DeploymentScale是否发生变化, 发生变化则添加到队列中, 队列中找不到也添加到队列中
	if deploymentScale, ok := r.DeploymentScaleQueue[deploymentScaleK8s.Name]; ok {
		if reflect.DeepEqual(deploymentScale.Spec, deploymentScaleK8s.Spec) {
			operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务没有发生变化!", req.NamespacedName.Name)
			return ctrl.Result{}, err
		}
	}
	// 把任务添加到队列中
	operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务发生变化, 添加到队列中!", req.NamespacedName.Name)
	if !r.IsStopTask(deploymentScaleK8s.Spec.EndTime) {
		r.AddQueue(deploymentScaleK8s)
	} else {
		r.DeploymentShrink()
	}
	return ctrl.Result{}, nil
}

// 从队列中删除任务
func (r *DeploymentScaleReconciler) DeleteQueue(deploymentScale *operatorcodehorsecomv1beta1.DeploymentScale) {
	delete(r.DeploymentScaleQueue, deploymentScale.Name)
	r.StopLoopTask()
	go r.RunLoopTask()
}

// 从队列中添加任务
func (r *DeploymentScaleReconciler) AddQueue(deploymentScale *operatorcodehorsecomv1beta1.DeploymentScale) {
	if r.DeploymentScaleQueue == nil {
		r.DeploymentScaleQueue = make(map[string]*operatorcodehorsecomv1beta1.DeploymentScale)
	}
	r.DeploymentScaleQueue[deploymentScale.Name] = deploymentScale
	r.StopLoopTask()
	go r.RunLoopTask()
}

// 执行任务
func (r *DeploymentScaleReconciler) RunLoopTask() {
	for _, deploymentScale := range r.DeploymentScaleQueue {
		if !deploymentScale.Spec.Active {
			operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务没有开启!", deploymentScale.Name)
			deploymentScale.Status.Active = false
			// 更新状态
			r.UpdateStatus(deploymentScale)
			continue
		}
		// 修改DeploymentScale的状态
		deploymentScale.Status.Active = true
		deploymentScale.Status.NextTime = int64(r.GetDelaySeconds(deploymentScale.Spec.StartTime).Seconds())
		// 更新状态
		r.UpdateStatus(deploymentScale)
		// 创建定时器
		deplay := r.GetDelaySeconds(deploymentScale.Spec.StartTime)
		if deplay.Hours() >= 1 {
			operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务在%.1f小时后开始执行", deploymentScale.Name, deplay.Hours())
		} else {
			operatorcodehorsecomv1beta1.L().Info().Msgf("[%s]任务在%.1f分钟后开始执行", deploymentScale.Name, deplay.Minutes())
		}
		ticker := time.NewTicker(deplay)
		r.Tickers = append(r.Tickers, ticker)
		r.Wg.Add(1)
		go func(deploymentScale *operatorcodehorsecomv1beta1.DeploymentScale) {
			defer r.Wg.Done()
			for {
				<-ticker.C
				// 重置定时器
				ticker.Reset(time.Second * time.Duration(deploymentScale.Spec.Period))
				// 修改任务状态
				deploymentScale.Status.Active = true
				deploymentScale.Status.NextTime = r.GetNextTime(float64(deploymentScale.Spec.Period)).Unix()
				// 执行任务逻辑
				err := r.DeploymentScale(deploymentScale)
				if err != nil {
					deploymentScale.Status.LastResult = fmt.Sprintf("扩容[%s] deployment失败, 原因: %s", deploymentScale.Name, err.Error())
				} else {
					deploymentScale.Status.LastResult = fmt.Sprintf("扩容[%s] deployment成功!", deploymentScale.Name)
				}
				// 更新状态
				r.UpdateStatus(deploymentScale)
			}
		}(deploymentScale)
	}
	r.Wg.Wait()
}

// 扩容deployments
func (r *DeploymentScaleReconciler) DeploymentScale(deploymentScale *operatorcodehorsecomv1beta1.DeploymentScale) error {
	for _, deploy := range deploymentScale.Spec.Deployments {
		namespaceName := types.NamespacedName{
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
		}
		deploymentK8s := &appsv1.Deployment{}
		err := r.Client.Get(context.TODO(), namespaceName, deploymentK8s)
		if err != nil {
			operatorcodehorsecomv1beta1.L().Error().Msgf("获取[%s]出错!", deploymentK8s.Name)
			return err
		}
		if r.DeploymentK8sMap == nil {
			r.DeploymentK8sMap = make(map[string]*appsv1.Deployment)
		}
		r.DeploymentK8sMap[deploymentK8s.Name] = deploymentK8s
		deploymentK8s.Spec.Replicas = &deploymentScale.Spec.Replicas
		err = r.Client.Update(context.TODO(), deploymentK8s)
		if err != nil {
			operatorcodehorsecomv1beta1.L().Error().Msgf("更新[%s]出错!", deploymentK8s.Name)
			return err
		}
	}
	return nil
}

// 缩容deployments
func (r *DeploymentScaleReconciler) DeploymentShrink() {
	for _, deploy := range r.DeploymentK8sMap {
		namespaceName := types.NamespacedName{
			Name:      deploy.Name,
			Namespace: deploy.Namespace,
		}
		deploymentK8s := &appsv1.Deployment{}
		err := r.Client.Get(context.TODO(), namespaceName, deploymentK8s)
		if err != nil {
			operatorcodehorsecomv1beta1.L().Error().Msgf("[%s]deployment查询失败, 原因: %s", deploy.Name, err.Error())
			return
		}
		deploymentK8s.Spec.Replicas = deploy.Spec.Replicas
		err = r.Client.Update(context.TODO(), deploymentK8s)
		if err != nil {
			operatorcodehorsecomv1beta1.L().Error().Msgf("[%s]deployment缩容失败, 原因: %s", deploy.Name, err.Error())
			return
		}
	}
}

// 获取任务的启动时间
func (r *DeploymentScaleReconciler) GetDelaySeconds(startTime string) time.Duration {
	times := strings.Split(startTime, ":")
	expectedHour, _ := strconv.Atoi(times[0])
	expectedMin, _ := strconv.Atoi(times[1])
	now := time.Now().Truncate(time.Second)
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowDate := todayDate.Add(time.Hour * time.Duration(24))
	curDuration := time.Hour*time.Duration(now.Hour()) + time.Minute*time.Duration(now.Minute())
	expectedDuration := time.Hour*time.Duration(expectedHour) + time.Minute*time.Duration(expectedMin)
	var seconds int
	if curDuration <= expectedDuration {
		seconds = int(todayDate.Add(expectedDuration).Sub(now).Seconds())
	} else {
		seconds = int(tomorrowDate.Add(expectedDuration).Sub(now).Seconds())
	}
	return time.Second * time.Duration(seconds)
}

// 获取NextTime
func (r *DeploymentScaleReconciler) GetNextTime(seconds float64) time.Time {
	return time.Now().Add(time.Second * time.Duration(seconds))
}

// 获取时间延迟
func (r *DeploymentScaleReconciler) IsStopTask(endTime string) bool {
	times := strings.Split(endTime, ":")
	endHour, _ := strconv.Atoi(times[0])
	endMin, _ := strconv.Atoi(times[1])
	now := time.Now()
	curDuration := time.Hour*time.Duration(now.Hour()) + time.Minute*time.Duration(now.Minute())
	endDuration := time.Hour*time.Duration(endHour) + time.Minute*time.Duration(endMin)
	if (curDuration - endDuration) >= 0 {
		return true
	} else {
		return false
	}
}

// 停止任务
func (r *DeploymentScaleReconciler) StopLoopTask() {
	for _, ticker := range r.Tickers {
		if ticker != nil {
			ticker.Stop()
		}
	}
}

// 更新DeploymentScale状态
func (r *DeploymentScaleReconciler) UpdateStatus(deploymentScale *operatorcodehorsecomv1beta1.DeploymentScale) {
	r.Lock.Lock()
	defer r.Lock.Unlock()
	deploymentScaleK8s := operatorcodehorsecomv1beta1.NewDeploymentScale()
	namespaceName := types.NamespacedName{
		Namespace: deploymentScale.Namespace,
		Name:      deploymentScale.Name,
	}
	err := r.Client.Get(context.TODO(), namespaceName, deploymentScaleK8s)
	if err != nil {
		operatorcodehorsecomv1beta1.L().Error().Msgf("[%s]任务执行出错, 原因: %s", namespaceName.Name, err.Error())
		return
	}
	deploymentScaleK8s.Status = deploymentScale.Status
	err = r.Client.Status().Update(context.TODO(), deploymentScaleK8s)
	if err != nil {
		operatorcodehorsecomv1beta1.L().Error().Msgf("[%s]任务更新状态出错, 原因: %s", namespaceName.Name, err.Error())
		return
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorcodehorsecomv1beta1.DeploymentScale{}).
		Complete(r)
}
