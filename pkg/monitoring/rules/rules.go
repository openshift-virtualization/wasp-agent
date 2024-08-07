package rules

import (
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	"github.com/openshift-virtualization/wasp-agent/pkg/monitoring/rules/alerts"
	utils2 "github.com/openshift-virtualization/wasp-agent/pkg/util"

	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
)

func SetupRules() error {
	return alerts.Register()
}

func CreatePrometheusRule(ruleName, namespace string) *promv1.PrometheusRule {
	rules, err := operatorrules.BuildPrometheusRule(
		ruleName,
		namespace,
		resources.WithLabels(make(map[string]string), utils2.DaemonSetLabels),
	)
	if err != nil {
		log.Log.Errorf("Failed to create PrometheusRule: %v", err)
		return nil
	}

	return rules
}
