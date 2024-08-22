package operator

import (
	"fmt"

	"github.com/openshift-virtualization/wasp-agent/pkg/monitoring/rules"
	utils2 "github.com/openshift-virtualization/wasp-agent/pkg/util"

	secv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	roleName        = "wasp"
	clusterRoleName = roleName + "-cluster"
	promRuleName    = "wasp-rules"
)

func getClusterPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"nodes",
			},
			Verbs: []string{
				"watch",
				"list",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"delete",
				"watch",
				"list",
			},
		},
	}
	rules = append(rules)
	return rules
}

func createClusterRole() *rbacv1.ClusterRole {
	return utils2.ResourceBuilder.CreateOperatorClusterRole(clusterRoleName, getClusterPolicyRules())
}

func createClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return utils2.ResourceBuilder.CreateOperatorClusterRoleBinding(utils2.OperatorServiceAccountName, clusterRoleName, utils2.OperatorServiceAccountName, namespace)
}

func createClusterRBAC(args *FactoryArgs) []client.Object {
	return []client.Object{
		createClusterRole(),
		createClusterRoleBinding(args.NamespacedArgs.Namespace),
	}
}
func createNamespacedRBAC(args *FactoryArgs) []client.Object {
	return []client.Object{
		createServiceAccount(args.NamespacedArgs.Namespace),
		CreateSCC(args.NamespacedArgs.Namespace, utils2.OperatorServiceAccountName),
	}
}
func createServiceAccount(namespace string) *corev1.ServiceAccount {
	return utils2.ResourceBuilder.CreateOperatorServiceAccount(utils2.OperatorServiceAccountName, namespace)
}

func createPrometheusRule(args *FactoryArgs) []client.Object {
	if args.NamespacedArgs.DeployPrometheusRule == "true" {
		return []client.Object{
			rules.CreatePrometheusRule(promRuleName, args.NamespacedArgs.Namespace),
		}
	}

	return nil
}

func createDaemonSet(args *FactoryArgs) []client.Object {
	return []client.Object{
		createWaspDaemonSet(args.NamespacedArgs.Namespace,
			args.NamespacedArgs.SwapUtilizationThresholdFactor,
			args.NamespacedArgs.MaxAverageSwapInPagesPerSecond,
			args.NamespacedArgs.MaxAverageSwapOutPagesPerSecond,
			args.NamespacedArgs.AverageWindowSizeSeconds,
			args.NamespacedArgs.Verbosity,
			args.Image,
			args.NamespacedArgs.PullPolicy),
	}
}

func createDaemonSetEnvVar(swapUtilizationTHresholdFactor, maxAverageSwapInPerSecond, maxAverageSwapOutPerSecond, averageWindowSizeSeconds, verbosity string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "SWAP_UTILIZATION_THRESHOLD_FACTOR",
			Value: swapUtilizationTHresholdFactor,
		},
		{
			Name:  "MAX_AVERAGE_SWAP_IN_PAGES_PER_SECOND",
			Value: maxAverageSwapInPerSecond,
		},
		{
			Name:  "MAX_AVERAGE_SWAP_OUT_PAGES_PER_SECOND",
			Value: maxAverageSwapOutPerSecond,
		},
		{
			Name:  "AVERAGE_WINDOW_SIZE_SECONDS",
			Value: averageWindowSizeSeconds,
		},
		{
			Name:  "VERBOSITY",
			Value: verbosity,
		},
		{
			Name:  "FSROOT",
			Value: "/host",
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func createWaspDaemonSet(namespace, swapUtilizationTHresholdFactor, maxAverageSwapInPagesPerSecond, maxAverageSwapOutPagesPerSecond, averageWindowSizeSeconds, verbosity, waspImage, pullPolicy string) *appsv1.DaemonSet {
	container := corev1.Container{
		Name:            "wasp-agent",
		Image:           waspImage,
		ImagePullPolicy: corev1.PullPolicy(pullPolicy),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("50M"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "host",
				MountPath: "/host",
			},
			{
				Name:      "rootfs",
				MountPath: "/rootfs",
			},
		},
	}
	container.Env = createDaemonSetEnvVar(swapUtilizationTHresholdFactor, maxAverageSwapInPagesPerSecond, maxAverageSwapOutPagesPerSecond, averageWindowSizeSeconds, verbosity)

	labels := resources.WithLabels(map[string]string{"name": "wasp"}, utils2.DaemonSetLabels)
	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wasp-agent",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "wasp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"description": "Configures swap for workloads",
					},
					Labels: map[string]string{
						"name": "wasp",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            "wasp",
					HostPID:                       true,
					HostUsers:                     boolPtr(true),
					TerminationGracePeriodSeconds: int64Ptr(5),
					Containers:                    []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: "host",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
						{
							Name: "rootfs",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
					},
					PriorityClassName: "system-node-critical",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "10%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{},
	}

	return ds
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func CreateSCC(saNamespace, saName string) *secv1.SecurityContextConstraints {
	scc := &secv1.SecurityContextConstraints{}
	userName := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName)

	scc = &secv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wasp",
			Namespace: saNamespace,
			Labels: map[string]string{
				"wasp.io": "",
			},
		},
		Users: []string{
			userName,
		},
	}
	setSCC(scc)

	return scc
}

func setSCC(scc *secv1.SecurityContextConstraints) {
	scc.AllowHostDirVolumePlugin = true
	scc.AllowHostIPC = true
	scc.AllowHostNetwork = true
	scc.AllowHostPID = true
	scc.AllowHostPorts = true
	scc.AllowPrivilegeEscalation = pointer.Bool(true)
	scc.AllowPrivilegedContainer = true
	scc.AllowedCapabilities = []corev1.Capability{
		"*",
	}
	scc.AllowedUnsafeSysctls = []string{
		"*",
	}
	scc.DefaultAddCapabilities = nil
	scc.RunAsUser = secv1.RunAsUserStrategyOptions{
		Type: secv1.RunAsUserStrategyRunAsAny,
	}
	scc.SELinuxContext = secv1.SELinuxContextStrategyOptions{
		Type: secv1.SELinuxStrategyRunAsAny,
	}
	scc.SeccompProfiles = []string{
		"*",
	}
	scc.SupplementalGroups = secv1.SupplementalGroupsStrategyOptions{
		Type: secv1.SupplementalGroupsStrategyRunAsAny,
	}
	scc.Volumes = []secv1.FSType{
		"*",
	}
}
