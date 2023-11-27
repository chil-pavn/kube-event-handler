/*
Copyright 2023 chil-pavn.

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
	"time"

	// "google.golang.org/appengine/log"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/chil-pavn/kube-event-operator/api/v1alpha1"
)

var logger = log.Log.WithName("controller_notifier")

// NotifierReconciler reconciles a Notifier object
type NotifierReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=api.chil-pavn.online,resources=notifiers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.chil-pavn.online,resources=notifiers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.chil-pavn.online,resources=notifiers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Notifier object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *NotifierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// TODO(user): your logic here
	log := logger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	log.Info("Reconcilecalled")
	notifier := &apiv1alpha1.Notifier{}
	err := r.Get(ctx, req.NamespacedName, notifier)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Scaler resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed")
		return ctrl.Result{}, err
	}

	for _, deploy := range notifier.Spec.Deployments {
		dep := &v1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		}, dep)
		if err != nil {
			return ctrl.Result{}, err
		}
		status := deploy.Status
		r.getPodStatus(ctx, dep)
		if err != nil {
			log.Error(err, "unable to fetch Pods Status for Deployment")
			return ctrl.Result{}, err
		}
		if status != "running" {
			log.Info("checking")
		}

	}

	return ctrl.Result{RequeueAfter: time.Duration(10 * time.Second)}, nil
}

func (r *NotifierReconciler) getPodStatus(ctx context.Context, deployment *v1.Deployment) (ctrl.Result, error) {

	podList, err := r.getPodsForDeployment(ctx, deployment)
	if err != nil {
		logger.Error(err, "unable to fetch Pods for Deployment")
		return ctrl.Result{}, err
	}

	// Process pods as needed
	for _, pod := range podList.Items {
		logger.Info("Pod Name", "Name", pod.Name)
		logger.Info("Pod Status", "Status", pod.Status.Phase)

		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			// TODO: If Pod is in Pending state , Failed, Unknown State
			return ctrl.Result{}, err
		}

		// Pod can be in Running phase but the container can go to Waiting Terminated etc
		containerCurrentState, containerLastState, err := r.fetchContainerStatus(ctx, pod)

		if err != nil {
			logger.Error(err, "unable to fetch the container state for the pod")
		}
		// Pod can be Running phase but the container can go to Waiting Terminated etc
		if containerCurrentState.Waiting != nil {
			// Delete the pod
			logger.Info("Deleting pod", "PodName", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete pod", "PodName", pod.Name)
				// Handle the error as needed
			} else {
				logger.Info("Pod deleted successfully", "PodName", pod.Name)
			}

			if containerLastState.Terminated != nil {
				logger.Info("Last State", "Terminated, Reason:", containerLastState.Terminated.Reason)
				if containerLastState.Terminated.Reason == "CrashLoopBackOff" {
					logger.Info("CRASHLOOPBACKOFF")
				}
				if containerLastState.Terminated.Reason == "OOMKilled" {
					logger.Info("Last State: ", "OOMKilled", "Will trigger scale up")
				}

			}
		}

		if containerCurrentState.Terminated != nil {
			logger.Info("This container is Terminated")
		}

	}
	// Add your custom logic to handle pod status as needed
	// For example, you could check if the pod is running, ready, etc.
	// You might want to trigger some action based on the pod status.
	return ctrl.Result{}, nil
}

func (r *NotifierReconciler) fetchContainerStatus(ctx context.Context, pod corev1.Pod) (corev1.ContainerState, corev1.ContainerState, error) {
	for _, container := range pod.Status.ContainerStatuses {
		containerName := container.Name
		containerState := container.State
		containerLastState := container.LastTerminationState

		logger.Info("Container Name", "Name", containerName)
		// logger.Info("Container Current State", "State", containerState)
		// logger.Info("Container Last State", "LastState", containerLastState)

		// Check if the container is not running
		if containerState.Running == nil {
			// Container is not running
			logger.Info("Container is not running")

			// Print state and reason
			if containerState.Waiting != nil {
				logger.Info("Container Waiting State", "Reason", containerState.Waiting.Reason)
				logger.Info("Container Waiting State", "Message", containerState.Waiting.Message)
				return containerState, containerLastState, nil
			} else if containerState.Terminated != nil {
				logger.Info("Container Terminated State", "Reason", containerState.Terminated.Reason)
				logger.Info("Container Terminated State", "Message", containerState.Terminated.Message)
				return containerState, containerLastState, nil
			}
		}
	}
	return corev1.ContainerState{}, corev1.ContainerState{}, nil
}

func (r *NotifierReconciler) getPodsForDeployment(ctx context.Context, deployment *v1.Deployment) (*corev1.PodList, error) {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	listOptions := &client.ListOptions{
		LabelSelector: selector,
		Namespace:     deployment.Namespace,
	}

	pods := &corev1.PodList{}
	err = r.List(ctx, pods, listOptions)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotifierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Notifier{}).
		Complete(r)
}
