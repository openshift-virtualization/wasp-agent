package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"
)

func nodeAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "NodeHighMemoryUtilization",
			Annotations: map[string]string{
				"description": "Memory utilization (RAM + Swap) at {{ $labels.instance }} is very high, exceeding 95% of total usable RAM. Ensure adequate memory resources are available to prevent performance degradation.",
				"summary":     "Memory utilization at {{ $labels.instance }} exceeds 95%.",
			},
			Expr: intstr.FromString("(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes + node_memory_SwapTotal_bytes - node_memory_SwapFree_bytes) / node_memory_MemTotal_bytes > 0.95"),
			For:  ptr.To(promv1.Duration("1m")),
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
			},
		},
		{
			Alert: "NodeCriticalMemoryUtilization",
			Annotations: map[string]string{
				"description": "Memory utilization (RAM + Swap) at {{ $labels.instance }} is critically high, exceeding 105% of total usable RAM. Immediate action is required to ensure system stability and prevent performance degradation.",
				"summary":     "Memory utilization at {{ $labels.instance }} exceeds 105%.",
			},
			Expr: intstr.FromString("(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes + node_memory_SwapTotal_bytes - node_memory_SwapFree_bytes) / node_memory_MemTotal_bytes > 1.05"),
			For:  ptr.To(promv1.Duration("1m")),
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "NodeHighSwapActivity",
			Annotations: map[string]string{
				"description": "High swap activity detected at {{ $labels.instance }}. The rate of swap out and swap in exceeds 200 in both operations in the last minute. This could indicate memory pressure and may affect system performance.",
				"summary":     "High swap activity detected at {{ $labels.instance }}.",
			},
			Expr: intstr.FromString("rate(node_vmstat_pswpout[1m]) > 200 and rate(node_vmstat_pswpin[1m]) > 200"),
			For:  ptr.To(promv1.Duration("1m")),
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
			},
		},
	}
}
