package taints

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openshift-virtualization/wasp-agent/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	WaspTaint = "waspEvictionTaint"
)

func GenerateTaint() v1.Taint {
	return v1.Taint{
		Key:    WaspTaint,
		Effect: v1.TaintEffectNoSchedule,
	}
}

func GenerateToleration() v1.Toleration {
	return v1.Toleration{
		Key:    WaspTaint,
		Effect: v1.TaintEffectNoSchedule,
	}
}

func NodeHasEvictionTaint(node *v1.Node) bool {
	// Check if the node has the specified taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == WaspTaint && taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func AddWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {

	taints := append(node.Spec.Taints, GenerateTaint())

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": taints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}

func RemoveWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {
	var newTaints []v1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != WaspTaint {
			newTaints = append(newTaints, taint)
		}
	}

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": newTaints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}
