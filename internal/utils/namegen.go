package utils

import "strings"

func CPJobNameGen(pod, c, ns string) string {
	return strings.Join([]string{"CPJob", c, pod, ns}, "-")
}

func CPPodNameGen(pod, ns string) string {
	return strings.Join([]string{"CPPod", pod, ns}, "-")
}

func CPCNameGen(pod, ns string) string {
	return strings.Join([]string{"CPContainer", pod, ns}, "-")
}
