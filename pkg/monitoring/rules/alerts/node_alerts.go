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
