package job

import (
	"fmt"

	kubeforenv1 "github.com/ggh41th/kubeforen/api/v1alpha1"
	"github.com/ggh41th/kubeforen/internal/utils"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

const (
	// I should consider the versionning of the image
	containerImage          = "curlimages/curl"
	CPPodServiceAccountName = "CPJobSA"
	CPCommand               = `curl -k -X POST "https://localhost:10250/checkpoint/%s/%s/%s -H "Authorization: Bearer %s"`
	SATokenPath             = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func CreateJob(cp *kubeforenv1.CheckPoint, controller, node string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.CPJobNameGen(cp.Spec.PodName, cp.Spec.ContainerName, cp.Spec.NameSpace),
			Namespace: cp.Spec.NameSpace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cp, schema.FromAPIVersionAndKind(kubeforenv1.GroupVersion.String(), cp.Kind)),
			},
		},
		Spec: batchv1.JobSpec{
			// We need to understand the different reason that could lead to criu
			// failing to find a good value for this field.
			BackoffLimit: ptr.To(int32(3)),
			// Give users enough time to debug if the job failed.
			TTLSecondsAfterFinished: ptr.To(int32(600)),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      utils.CPPodNameGen(cp.Spec.PodName, cp.Spec.NameSpace),
					Namespace: cp.Spec.NameSpace,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:    utils.CPCNameGen(cp.Spec.PodName, cp.Spec.NameSpace),
							Image:   containerImage,
							Command: []string{fmt.Sprintf(cp.Spec.Compression, cp.Spec.NameSpace, cp.Spec.PodName, cp.Spec.ContainerName, SATokenPath)},
						},
					},
					ServiceAccountName: CPPodServiceAccountName,
					NodeName:           node,
					HostNetwork:        true,
				},
			},
		},
	}
}
