package job

import (
	"errors"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func IsSuccessful(j *batchv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobSuccessCriteriaMet && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsTerminated(j *batchv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func IsFailed(j *batchv1.Job) (bool, error) {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true, errors.New(fmt.Sprintf("Job failed, %s", c.Message))
		}
	}
	return false, nil
}
