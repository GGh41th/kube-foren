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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubeforenv1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	CONTAINER_IMAGE   = "busybox"
	CONTAINER_COMMAND = "sleep 3600"
)

// Helper func to create a container checkpoint
func createCheckpoint(ctx context.Context, name, pod, container string) {
	chkpt := &kubeforenv1.CheckPoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: kubeforenv1.CheckPointSpec{
			PodName:       pod,
			ContainerName: container,
		},
	}

	Expect(k8sClient.Create(ctx, chkpt)).To(Succeed())
}

// Helper func to create a pod with a specified container name
func createPod(ctx context.Context, pod, container string) {
	podObj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    container,
					Image:   CONTAINER_IMAGE,
					Command: []string{CONTAINER_COMMAND},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, podObj)).To(Succeed())
}

var _ = Describe("CheckPoint Controller", func() {
	ctx := context.Background()

	It("Should set the finalizer", func() {
		const chkptName = "pending-chkpt"
		const podName = "pending-pod"
		const containerName = "pending-container"

		nn := types.NamespacedName{Namespace: "default", Name: chkptName}
		reconciler := CheckPointReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		createCheckpoint(ctx, chkptName, podName, containerName)

		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})).
			Error().ShouldNot(HaveOccurred())

		chkpt := &kubeforenv1.CheckPoint{}

		Expect(k8sClient.Get(ctx, nn, chkpt)).Should(Succeed())
		Expect(controllerutil.ContainsFinalizer(chkpt, finalizerName)).To(BeTrue())
	})

	It("Should skip terminal states", func() {
		const chkptFailed = "failed-chkpt"
		const podFailed = "failed-pod"
		const containerFailed = "failed-container"

		const chkptReady = "ready-chkpt"
		const podReady = "ready-pod"
		const containerReady = "ready-container"

		nnFailed := types.NamespacedName{Namespace: "default", Name: chkptFailed}
		nnReady := types.NamespacedName{Namespace: "default", Name: chkptReady}
		reconciler := CheckPointReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		createCheckpoint(ctx, chkptFailed, podFailed, containerFailed)
		createCheckpoint(ctx, chkptReady, podReady, containerReady)

		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nnFailed})).To(Equal(ctrl.Result{}))
		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nnReady})).To(Equal(ctrl.Result{}))

	})

	It("Should set phase to failed if pod doesnt exist", func() {
		const chkptName = "missingp-chkpt"
		const podName = "missingp-pod"
		const containerName = "missingp-container"

		nn := types.NamespacedName{Namespace: "default", Name: chkptName}
		reconciler := CheckPointReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		createCheckpoint(ctx, chkptName, podName, containerName)

		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})).
			Error().ShouldNot(HaveOccurred())

		chkpt := &kubeforenv1.CheckPoint{}

		Expect(k8sClient.Get(ctx, nn, chkpt)).Should(Succeed())
		Expect(chkpt.Status.Phase).To(Equal(kubeforenv1.ContainerCheckpointFailed))
	})

	//TODO: Move this test to e2e tests.
	It("Should set phase to failed if container doesnt exist", func() {
		const chkptName = "missingc-chkpt"
		const podName = "missingc-pod"
		const containerName = "missingc-container"

		nn := types.NamespacedName{Namespace: "default", Name: chkptName}
		reconciler := CheckPointReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		createCheckpoint(ctx, chkptName, podName, containerName)
		createPod(ctx, podName, "failed-pod1")

		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})).
			Error().ShouldNot(HaveOccurred())

		chkpt := &kubeforenv1.CheckPoint{}

		Expect(k8sClient.Get(ctx, nn, chkpt)).Should(Succeed())
		Expect(chkpt.Status.Phase).To(Equal(kubeforenv1.ContainerCheckpointFailed))
	})

	//TODO: Move this test to e2e tests.
	It("Should checkpoint container and set status to ready", func() {
		const chkptName = "successful-chkpt"
		const podName = "successful-pod"
		const containerName = "successful-container"

		nn := types.NamespacedName{Namespace: "default", Name: chkptName}
		reconciler := CheckPointReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		createCheckpoint(ctx, chkptName, podName, containerName)
		createPod(ctx, podName, containerName)

		Expect(reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})).
			Error().ShouldNot(HaveOccurred())

		chkpt := &kubeforenv1.CheckPoint{}

		Expect(k8sClient.Get(ctx, nn, chkpt)).Should(Succeed())
		Expect(chkpt.Status.Phase).To(Equal(kubeforenv1.ContainerCheckpointReady))
	})
})
