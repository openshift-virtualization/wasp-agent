package util

import (
	utils "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/resources"
)

const (
	// WaspLabel is the labe applied to all non operator resources
	WaspLabel = "wasp.io"
	// AppKubernetesManagedByLabel is the Kubernetes recommended managed-by label
	AppKubernetesManagedByLabel = "app.kubernetes.io/managed-by"
	// AppKubernetesComponentLabel is the Kubernetes recommended component label
	AppKubernetesComponentLabel = "app.kubernetes.io/component"
	OperatorServiceAccountName  = "wasp"
)

var commonLabels = map[string]string{
	WaspLabel:                   "",
	AppKubernetesManagedByLabel: "wasp",
	AppKubernetesComponentLabel: "swap",
}

var DaemonSetLabels = map[string]string{
	"wasp.io": "",
	"tier":    "node",
}

// ResourceBuilder helps in creating k8s resources
var ResourceBuilder = utils.NewResourceBuilder(commonLabels, DaemonSetLabels)
