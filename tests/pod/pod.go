package pod

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

func GetMemhogPod(podName string, ctnName string, res v1.ResourceRequirements) *v1.Pod {
	// Due to https://github.com/kubernetes/kubernetes/issues/115819,
	// When evictionHard to used, we were setting grace period to 0 which meant the default setting (30 seconds)
	// This could help with flakiness as we should send sigterm right away.
	var gracePeriod int64 = 1
	env := []v1.EnvVar{
		{
			Name: "MEMORY_LIMIT",
			ValueFrom: &v1.EnvVarSource{
				ResourceFieldRef: &v1.ResourceFieldSelector{
					Resource: "limits.memory",
				},
			},
		},
	}

	// If there is a limit specified, pass 80% of it for -mem-total, otherwise use the downward API
	// to pass limits.memory, which will be the total memory available.
	// This helps prevent a guaranteed pod from triggering an OOM kill due to it's low memory limit,
	// which will cause the test to fail inappropriately.
	var memLimit string
	if limit, ok := res.Limits[v1.ResourceMemory]; ok {
		memLimit = strconv.Itoa(int(
			float64(limit.Value()) * 0.8))
	} else {
		memLimit = "$(MEMORY_LIMIT)"
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy:                 v1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &gracePeriod,
			Containers: []v1.Container{
				{
					Name:            ctnName,
					Image:           "registry.k8s.io/e2e-test-images/agnhost:2.52",
					ImagePullPolicy: "Always",
					Env:             env,
					Args:            []string{"stress", "--mem-alloc-size", "500Mi", "--mem-alloc-sleep", "5s", "--mem-total", memLimit},
					Resources:       res,
				},
			},
		},
	}
}

// returns a pod that does not use any resources
func InnocentPod() *v1.Pod {
	// Due to https://github.com/kubernetes/kubernetes/issues/115819,
	// When evictionHard to used, we were setting grace period to 0 which meant the default setting (30 seconds)
	// This could help with flakiness as we should send sigterm right away.
	var gracePeriod int64 = 1
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "virt-launcher-innocent-pod"},
		Spec: v1.PodSpec{
			RestartPolicy:                 v1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &gracePeriod,
			Containers: []v1.Container{
				{
					Image: "registry.k8s.io/e2e-test-images/busybox:1.29-4",
					Name:  "innocent-container",
					Command: []string{
						"sh",
						"-c",
						"while true; do sleep 5; done",
					},
					Resources: v1.ResourceRequirements{
						// These values are set so that we don't consider this pod to be over the limits
						// If the requests are not set, then we assume a request limit of 0 so it is always over.
						// This fixes this for the innocent pod.
						Requests: v1.ResourceList{
							v1.ResourceEphemeralStorage: resource.MustParse("50Mi"),
							v1.ResourceMemory:           resource.MustParse("50Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceEphemeralStorage: resource.MustParse("50Mi"),
							v1.ResourceMemory:           resource.MustParse("50Mi"),
						},
					},
				},
			},
		},
	}
}
