package alerts

import (
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

const (
	partOfAlertLabelKey      = "kubernetes_operator_part_of"
	partOfAlertLabelValue    = "kubevirt"
	componentAlertLabelKey   = "kubernetes_operator_component"
	componentAlertLabelValue = "kubevirt"
	runbookAnnotationKey     = "runbook_url"
	runbookURLTemplate       = "https://github.com/openshift-virtualization/wasp-agent/tree/main/docs/runbooks/%s.md"
)

func Register() error {
	alerts := [][]promv1.Rule{
		nodeAlerts(),
	}

	for _, alertGroup := range alerts {
		for _, alert := range alertGroup {
			alert.Labels[partOfAlertLabelKey] = partOfAlertLabelValue
			alert.Labels[componentAlertLabelKey] = componentAlertLabelValue
			alert.Annotations[runbookAnnotationKey] = fmt.Sprintf(runbookURLTemplate, alert.Alert)
		}

	}

	return operatorrules.RegisterAlerts(alerts...)
}
