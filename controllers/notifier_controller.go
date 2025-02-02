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
	"fmt"
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
	log.Info("Reconcile called")
	
	notifier := &apiv1alpha1.Notifier{}
	if err := r.Get(ctx, req.NamespacedName, notifier); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Notifier resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Notifier")
		return ctrl.Result{}, err
	}

	for _, deploy := range notifier.Spec.Deployments {
		dep := &v1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		}, dep); err != nil {
			log.Error(err, "Failed to get Deployment", "deployment", deploy.Name)
			return ctrl.Result{}, err
		}

		if _, err := r.getPodStatus(ctx, dep); err != nil {
			log.Error(err, "Failed to get pod status", "deployment", deploy.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *NotifierReconciler) getPodStatus(ctx context.Context, deployment *v1.Deployment) (ctrl.Result, error) {

	podList, err := r.getPodsForDeployment(ctx, deployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to fetch pods for deployment: %w", err)
	}

	// Process pods as needed
	for _, pod := range podList.Items {
		logger.Info("Processing pod", "name", pod.Name, "status", pod.Status.Phase)

		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			failureMsg := fmt.Sprintf("Pod %s in namespace %s is in %s state", 
				pod.Name, pod.Namespace, pod.Status.Phase)
			if err := ProcessFailure(failureMsg); err != nil {
				logger.Error(err, "Failed to process pod failure notification",
					"pod", pod.Name,
					"namespace", pod.Namespace,
					"phase", pod.Status.Phase)
				continue
			}
		}

		// Pod can be in Running phase but the container can go to Waiting Terminated etc
		containerCurrentState, containerLastState, err := r.fetchContainerStatus(ctx, pod)

		if err != nil {
			logger.Error(err, "Failed to fetch container status", "pod", pod.Name)
			continue
		}

		if err := r.handleContainerState(ctx, pod, containerCurrentState, containerLastState); err != nil {
			logger.Error(err, "Failed to handle container state", "pod", pod.Name)
			continue
		}
	}

	return ctrl.Result{}, nil
}

// New helper function to handle container state
func (r *NotifierReconciler) handleContainerState(ctx context.Context, pod corev1.Pod, currentState, lastState corev1.ContainerState) error {
	if currentState.Waiting != nil {
		// Delete the pod
		if err := r.Delete(ctx, &pod); err != nil {
			return fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
		}
		logger.Info("Pod deleted successfully", "podName", pod.Name)

		failureMsg := fmt.Sprintf("Container in pod %s is in waiting state. Reason: %s, Message: %s",
			pod.Name, currentState.Waiting.Reason, currentState.Waiting.Message)
		return ProcessFailure(failureMsg)
	}

	if currentState.Terminated != nil {
		failureMsg := fmt.Sprintf("Container in pod %s is terminated. Reason: %s, Message: %s",
			pod.Name, currentState.Terminated.Reason, currentState.Terminated.Message)
		return ProcessFailure(failureMsg)
	}

	if lastState.Terminated != nil {
		switch lastState.Terminated.Reason {
		case "CrashLoopBackOff":
			return ProcessFailure(fmt.Sprintf("Pod %s is in CrashLoopBackOff state", pod.Name))
		case "OOMKilled":
			return ProcessFailure(fmt.Sprintf("Pod %s was OOMKilled", pod.Name))
		}
	}
	// Add your custom logic to handle pod status as needed
    // For example, you could check if the pod is running, ready, etc.
    // You might want to trigger some action based on the pod status.
	return nil
}

func (r *NotifierReconciler) fetchContainerStatus(_ context.Context, pod corev1.Pod) (corev1.ContainerState, corev1.ContainerState, error) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return corev1.ContainerState{}, corev1.ContainerState{}, fmt.Errorf("no container statuses found for pod %s", pod.Name)
	}

	// We'll check the first container's status
	container := pod.Status.ContainerStatuses[0]
	
	logger.Info("Container status", 
		"pod", pod.Name,
		"container", container.Name,
		"ready", container.Ready,
		"restartCount", container.RestartCount)

	return container.State, container.LastTerminationState, nil
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
