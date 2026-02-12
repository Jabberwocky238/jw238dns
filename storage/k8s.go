package storage

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewK8sClient creates a Kubernetes client using in-cluster configuration.
// This function must be called from within a Kubernetes pod.
func NewK8sClient() (kubernetes.Interface, error) {
	// Use in-cluster config (required for running inside K8s)
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}
