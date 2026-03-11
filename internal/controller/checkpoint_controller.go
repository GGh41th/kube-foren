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
	"errors"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kubeforenv1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	kubeforenv1alpha1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	"github.com/ggh41th/kubeforen/internal/job"
	"github.com/ggh41th/kubeforen/internal/utils"
)

const (
	finalizerName = "kubeforen.org/checkpoint-finalizer"
)

// CheckPointReconciler reconciles a CheckPoint object
type CheckPointReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeforen.org,resources=checkpoints/finalizers,verbs=update

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
	// When a Checkpoint is being deleted , we need to ensure that all the ?? are
	// deleted
	if cp.DeletionTimestamp != nil {

		if err := r.reconcileDelete(ctx, cp); err != nil {
			log.Error(err, "Failed to delete Checkpoint ressource")
			return ctrl.Result{}, err
		}

		// Removie finalizers from the object if they exist
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
		patch := client.MergeFrom(cp)
		if updated := controllerutil.AddFinalizer(cp, finalizerName); updated {
			if err := r.Patch(ctx, cp, patch); err != nil {
				log.Error(err, "Failed to add finalizer", "finalizer", finalizerName)
				return ctrl.Result{}, err
			}
		}
	}

	// take a copy of the object , since we'll update it below (we will update the status for whatever reason , using a defer function)
	//	orig:=cp.DeepCopy()

	// we will set the conditions , which i have no idea yet what they will about

	return r.reconcile(ctx, cp)
}

func (r *CheckPointReconciler) reconcileDelete(ctx context.Context, cp *kubeforenv1.CheckPoint) (ctrl.Result, error) {

}

func (r *CheckPointReconciler) reconcile(ctx context.Context, cp *kubeforenv1.CheckPoint) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues(
		"Checkpoint", cp.Name,
		"Pod", cp.Spec.PodName,
		"Namespace", cp.Spec.NameSpace,
	)
	// retrieve the pod to checkpoint
	p := &corev1.Pod{}
	pName := types.NamespacedName{
		Name:      cp.Spec.PodName,
		Namespace: cp.Spec.NameSpace,
	}

	if err := r.Get(ctx, pName, p); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Requested pod for checkpointing does not exist", "Pod")
			// Pod doesn't exist , do not requeue until the user fixes the ressource
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch pod")
		return ctrl.Result{}, err
	}

	c, err := utils.ExtractContainer(p.Spec.Containers, cp.Spec.ContainerName)
	if err != nil {
		log.Error(err, "Requested container does not exist in pod", "Container", cp.Spec.ContainerName)
		// Requested container does not exist , do not requeue and wait for the user to fix the ressource
		return ctrl.Result{}, nil
	}

	node := p.Spec.NodeName
	if node == "" {
		log.Error(errors.New("Node name field is empty"), "Requested pod for checkpointing have not been scheduled yet")
		// the only possible reason for a missing node name field is that the pod wasn't
		// scheduled yet.
		// (TODO: the one minute period is somewhat random , find a better way to handle
		//  this scenario).
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	j := &batchv1.Job{}
	jName := types.NamespacedName{
		Name:      utils.CPJobNameGen(cp.Spec.PodName, c, cp.Spec.NameSpace),
		Namespace: cp.Spec.NameSpace,
	}
	if err := r.Get(ctx, jName, j); err != nil {
		if apierrors.IsAlreadyExists(err) {
			success, err := r.checkJobSuccess(j)
			if !success {
				if err == nil {
					// Job hasn't completed yet , do not requeue.
					return ctrl.Result{}, nil
				}
				log.Error(err, "Could not run checkpointing job successfully")
				// At this level , it is either a kubelet or a criu issue , eitherway
				// we can't solve it , let the user fix it and retry.
				return ctrl.Result{}, nil
			}

		}
		log.Error(err, "Could not fetch job", "Job", j.Name)
		return ctrl.Result{}, err
	}
	j = job.CreateJob(cp, c, node)
	if err := r.Create(ctx, j); err != nil {
		log.Error(err, "Failed to create job for checkpointing", "pod", cp.Spec.PodName, "namespace", cp.Spec.NameSpace)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil

}

func (r *CheckPointReconciler) checkJobSuccess(j *batchv1.Job) (bool, error) {
	if !job.IsTerminated(j) {
		// job hasn't completed yet.
		return false, nil
	}
	if job.IsSuccessful(j) {
		return true, nil
	}
	if fail, err := job.IsFailed(j); fail {
		return false, err
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheckPointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeforenv1alpha1.CheckPoint{}).
		Owns(&batchv1.Job{}).
		Named("checkpoint").
		Complete(r)
}
