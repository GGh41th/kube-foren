package utils

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
)

func ExtractContainer(cs []corev1.Container, c string) (string, error) {
	// If the pod checkpoint KEP was accepted , an empty container name will
	// signal a podSnapshot
	// We return the first container if the user didn't specify a container
	// name
	if c == "" {
		if len(cs) == 0 {
			return "", errors.New("Pod does not have containers , can't checkpoint")
		}
		return cs[0].Name, nil
	}

	for _, cont := range cs {
		if cont.Name == c {
			return c, nil
		}
	}
	return "", errors.New("Requested Container isn't present in the pod")
}
