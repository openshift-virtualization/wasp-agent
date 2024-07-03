package client

//go:generate mockgen -source $GOFILE -package=$GOPACKAGE -destination=generated_mock_$GOFILE

/*
 ATTENTION: Rerun code generators when interface signatures are modified.
*/

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubevirtclient "kubevirt.io/application-aware-quota/pkg/generated/kubevirt/clientset/versioned"
)

type WaspClient interface {
	RestClient() *rest.RESTClient
	kubernetes.Interface
	KubevirtClient() kubevirtclient.Interface
	DiscoveryClient() discovery.DiscoveryInterface
	Config() *rest.Config
}

type wasp struct {
	master          string
	kubeconfig      string
	restClient      *rest.RESTClient
	config          *rest.Config
	kubevirtClient  *kubevirtclient.Clientset
	discoveryClient *discovery.DiscoveryClient
	dynamicClient   dynamic.Interface
	*kubernetes.Clientset
}

func (k wasp) KubevirtClient() kubevirtclient.Interface {
	return k.kubevirtClient
}

func (k wasp) Config() *rest.Config {
	return k.config
}

func (k wasp) RestClient() *rest.RESTClient {
	return k.restClient
}
func (k wasp) DiscoveryClient() discovery.DiscoveryInterface {
	return k.discoveryClient
}
