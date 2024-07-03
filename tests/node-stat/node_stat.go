package node_stat

import (
	"bytes"
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"kubevirt.io/wasp/tests/framework"
	"strconv"
	"strings"
)

func getMemInfoByString(f *framework.Framework, node v1.Node, field string, fileToGrep string) (int, error) {
	stdout, stderr, err := executeCommandOnNodeThroughWaspDaemonSet(f, node.Name, []string{"grep", field, fileToGrep})
	if err != nil {
		return 0, fmt.Errorf("high level err:%v  stderr in the node: %v", err.Error(), stderr)
	}
	fields := strings.Fields(stdout)
	size, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, err
	}
	return size, nil
}

func GetAvailableMemSizeInKib(f *framework.Framework, node v1.Node) (int, error) {
	return getMemInfoByString(f, node, "MemAvailable", "/proc/meminfo")
}

func GetSwapInPages(f *framework.Framework, node v1.Node) (int, error) {
	swapin, err := getMemInfoByString(f, node, "pswpin", "/proc/vmstat")
	return swapin * 4 * 1024, err
}
func GetSwapOutPages(f *framework.Framework, node v1.Node) (int, error) {
	swapout, err := getMemInfoByString(f, node, "pswpout", "/proc/vmstat")
	return swapout * 4 * 1024, err
}

func executeCommandOnNodeThroughWaspDaemonSet(f *framework.Framework, nodeName string, command []string) (stdout, stderr string, err error) {
	waspPod, err := getPodsOnNodeByDaemonSet(f, nodeName)
	if err != nil {
		return "", "", err
	}
	return ExecuteCommandOnPodWithResults(f, waspPod, "wasp-agent", command)
}

func getPodsOnNodeByDaemonSet(f *framework.Framework, nodeName string) (*v1.Pod, error) {
	daemonSet, err := f.K8sClient.AppsV1().DaemonSets(f.WaspNamespace).Get(context.TODO(), "wasp-agent", metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get DaemonSet: %v", err)
	}

	podList, err := f.K8sClient.CoreV1().Pods(f.WaspNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var pods []v1.Pod
	// Filter pods by node name and owner reference
	for _, pod := range podList.Items {
		if pod.Spec.NodeName == nodeName && isOwnedByDaemonSet(&pod, daemonSet) {
			pods = append(pods, pod)
		}
	}
	if len(pods) != 1 {
		return nil, fmt.Errorf("expect one wasp pod to exist on the node")
	}

	return &pods[0], nil
}

func isOwnedByDaemonSet(pod *v1.Pod, daemonSet *appsv1.DaemonSet) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" && ownerRef.UID == daemonSet.UID {
			return true
		}
	}
	return false
}

func ExecuteCommandOnPodWithResults(f *framework.Framework, pod *v1.Pod, containerName string, command []string) (stdout, stderr string, err error) {
	var (
		stdoutBuf bytes.Buffer
		stderrBuf bytes.Buffer
	)
	options := remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Tty:    false,
	}
	err = ExecuteCommandOnPodWithOptions(f, pod, containerName, command, options)
	return stdoutBuf.String(), stderrBuf.String(), err
}

func ExecuteCommandOnPodWithOptions(f *framework.Framework, pod *v1.Pod, containerName string, command []string, options remotecommand.StreamOptions) error {
	req := f.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("container", containerName)

	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	virtConfig, err := clientcmd.BuildConfigFromFlags(f.KubeURL, f.KubeConfig)
	if err != nil {
		return err
	}

	executor, err := remotecommand.NewSPDYExecutor(virtConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	return executor.Stream(options)
}
