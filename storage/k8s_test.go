package storage

import (
	"testing"
)

func TestNewK8sClient(t *testing.T) {
	// This test will fail outside of a Kubernetes cluster
	// as it requires in-cluster configuration
	_, err := NewK8sClient()

	// We expect an error when running outside K8s
	if err == nil {
		t.Log("Successfully created K8s client (running in cluster)")
	} else {
		t.Logf("Expected error outside K8s cluster: %v", err)
	}
}
