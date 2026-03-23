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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubeforenv1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	"github.com/ggh41th/kubeforen/internal/utils"
)

const (
	finalizerName = "kubeforen.org/checkpoint-finalizer"
)

// CheckPointReconciler reconciles a CheckPoint object
type CheckPointReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	KubeClient kubernetes.Interface
}

type checkpointResponse struct {
	Items []string `json:"items"`
}

// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=create

func (r *CheckPointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("Checkpoint", req.Name, "Namespace", req.Namespace)
	// Fetch CheckPoint instance
	cp := new(kubeforenv1.CheckPoint)
	if err := r.Get(ctx, req.NamespacedName, cp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Resource might been deleted")
			// object not found , do nothing and return.
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch ressource")

		// Error Reading the object , requeue.
		return ctrl.Result{}, err
	}

	// Handle deletion
	if cp.DeletionTimestamp != nil {
		if err := r.reconcileDelete(ctx, cp); err != nil {
			log.Error(err, "Failed to delete Checkpoint ressource")
			return ctrl.Result{}, err
		}

		// Remove finalizers from the object if they exist
		patch := client.MergeFrom(cp.DeepCopy())
		if updated := controllerutil.RemoveFinalizer(cp, finalizerName); updated {
			if err := r.Patch(ctx, cp, patch); err != nil {
				log.Error(err, "Failed to remove finalizer", "finalizer", finalizerName)
				return ctrl.Result{}, err
			}
		}
	}

	// Add finalizers if missing
	if contains := controllerutil.ContainsFinalizer(cp, finalizerName); !contains {
		patch := client.MergeFrom(cp.DeepCopy())
		if updated := controllerutil.AddFinalizer(cp, finalizerName); updated {
			if err := r.Patch(ctx, cp, patch); err != nil {
				log.Error(err, "Failed to add finalizer", "finalizer", finalizerName)
				return ctrl.Result{}, err
			}
			log.Info("Added fianlizer")
		}
	}

	return r.reconcile(ctx, cp)
}

func (r *CheckPointReconciler) reconcile(ctx context.Context, cp *kubeforenv1.CheckPoint) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues(
		"Checkpoint", cp.Name,
		"Pod", cp.Spec.PodName,
		"Container", cp.Spec.ContainerName,
		"Namespace", cp.Spec.NameSpace,
	)

	// Skip terminal states only (Ready and Failed).
	if cp.Status.Phase == kubeforenv1.ContainerCheckpointReady ||
		cp.Status.Phase == kubeforenv1.ContainerCheckpointFailed {
		return ctrl.Result{}, nil
	}

	// Set the initial state (Pending).
	if cp.Status.Phase == "" {
		if err := r.setPending(ctx, cp, "Task submitted, waiting to start checkpointing"); err != nil {
			return ctrl.Result{}, err
		}
	}

	// retrieve the pod to checkpoint
	p := &corev1.Pod{}
	pName := types.NamespacedName{
		Name:      cp.Spec.PodName,
		Namespace: cp.Spec.NameSpace,
	}

	if err := r.Get(ctx, pName, p); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Requested pod for checkpointing does not exist")
			// Pod doesn't exist , requeue only if failed to update status.
			return ctrl.Result{}, r.setFailed(ctx, cp, "Requested pod does not exist")
		}
		log.Error(err, "Failed to fetch pod")

		return ctrl.Result{}, err
	}

	node := p.Spec.NodeName
	if node == "" {
		// Possibly the pod hasn't been scheduled yet, we wait for some time and retry.
		log.Error(errors.New("Node name field is empty"), "Requested pod for checkpointing have not been scheduled yet")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	cp.Status.NodeName = node

	c, err := utils.ExtractContainer(p.Spec.Containers, cp.Spec.ContainerName)
	if err != nil {
		log.Error(err, "Requested container does not exist in pod", "Container", cp.Spec.ContainerName)
		// Requested container does not exist , do not requeue.
		return ctrl.Result{}, r.setFailed(ctx, cp, "Requested container not found")
	}

	item, err := r.checkpointContainer(ctx, p.Namespace, p.Name, c, node)

	if err != nil {
		return ctrl.Result{}, r.setFailed(ctx, cp, err.Error())
	}

	return ctrl.Result{}, r.setReady(ctx, cp, item, "Container checkpointed successfully")

}

func (r *CheckPointReconciler) reconcileDelete(ctx context.Context, cp *kubeforenv1.CheckPoint) error {
	// For now we won't be deleting the actual checkpoints, we leave that to an external program.
	return nil
}

func (r *CheckPointReconciler) setFailed(ctx context.Context, cp *kubeforenv1.CheckPoint, message string) error {
	cp.Status.Phase = kubeforenv1.ContainerCheckpointFailed
	meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
		Type:               kubeforenv1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "CheckpointFailed",
		Message:            message,
		ObservedGeneration: cp.Generation,
	})
	return r.Status().Update(ctx, cp)
}

func (r *CheckPointReconciler) setPending(ctx context.Context, cp *kubeforenv1.CheckPoint, message string) error {
	cp.Status.Phase = kubeforenv1.ContainerCheckpointPending
	meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
		Type:               kubeforenv1.ConditionReady,
		Status:             metav1.ConditionUnknown,
		Reason:             "CheckpointPending",
		Message:            message,
		ObservedGeneration: cp.Generation,
	})
	return r.Status().Update(ctx, cp)
}

func (r *CheckPointReconciler) setReady(ctx context.Context, cp *kubeforenv1.CheckPoint, item, message string) error {
	cp.Status.Phase = kubeforenv1.ContainerCheckpointReady
	cp.Status.CheckPointName = item
	meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
		Type:               kubeforenv1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "CheckpointReady",
		Message:            message,
		ObservedGeneration: cp.Generation,
	})
	return r.Status().Update(ctx, cp)
}
func (r *CheckPointReconciler) checkpointContainer(ctx context.Context, ns, p, c, n string) (string, error) {

	req := r.KubeClient.CoreV1().RESTClient().Post().Resource("nodes").Name(n).SubResource("proxy", "checkpoint", ns, p, c)
	res := req.Do(ctx)

	data, err := res.Raw()

	if err != nil {
		return "", err
	}

	var cres checkpointResponse

	err = json.Unmarshal(data, &cres)
	if err != nil {
		return "", err
	}

	if len(cres.Items) == 0 {
		return "", errors.New("Checkpoint response contains no items")
	}
	return cres.Items[0], nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheckPointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeforenv1.CheckPoint{}).
		Named("checkpoint").
		Complete(r)
}
