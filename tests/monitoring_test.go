package tests

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/openshift-virtualization/wasp-agent/tests/framework"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Prometheus Rules", func() {
	f := framework.NewFramework("prometheus-rules-test")

	Context("Alert Rules", func() {
		It("should have the required annotations and labels", func() {
			prometheusRule := getPrometheusRule(f)
			for _, group := range prometheusRule.Spec.Groups {
				for _, rule := range group.Rules {
					if len(rule.Alert) > 0 {
						Expect(rule.Annotations).ToNot(BeNil())
						checkRequiredAnnotations(rule)

						Expect(rule.Labels).ToNot(BeNil())
						checkRequiredLabels(rule)
					}
				}
			}
		})
	})
})

func getPrometheusRule(f *framework.Framework) *monitoringv1.PrometheusRule {
	By("Wait for wasp-rules")
	promRule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wasp-rules",
			Namespace: "wasp",
		},
	}
	Eventually(func() error {
		return f.CrClient.Get(context.TODO(), crclient.ObjectKeyFromObject(promRule), promRule)
	}, 5*time.Minute, 1*time.Second).Should(BeNil())
	return promRule
}

func checkRequiredAnnotations(rule monitoringv1.Rule) {
	ExpectWithOffset(1, rule.Annotations).To(HaveKeyWithValue("summary", Not(BeEmpty())),
		"%s summary is missing or empty", rule.Alert)
	ExpectWithOffset(1, rule.Annotations).To(HaveKey("runbook_url"),
		"%s runbook_url is missing", rule.Alert)
	ExpectWithOffset(1, rule.Annotations).To(HaveKeyWithValue("runbook_url", ContainSubstring(rule.Alert)),
		"%s runbook_url doesn't include alert name", rule.Alert)

	resp, err := http.Head(rule.Annotations["runbook_url"])
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), fmt.Sprintf("%s runbook is not available", rule.Alert))
	ExpectWithOffset(1, resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("%s runbook is not available", rule.Alert))
}

func checkRequiredLabels(rule monitoringv1.Rule) {
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("severity", BeElementOf("info", "warning", "critical")),
		"%s severity label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("operator_health_impact", BeElementOf("none", "warning", "critical")),
		"%s operator_health_impact label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("kubernetes_operator_part_of", "kubevirt"),
		"%s kubernetes_operator_part_of label is missing or not valid", rule.Alert)
	ExpectWithOffset(1, rule.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "kubevirt"),
		"%s kubernetes_operator_component label is missing or not valid", rule.Alert)
}
