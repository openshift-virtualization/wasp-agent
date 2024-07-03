package pod_evictor

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/log"
)

// PodEvictor is an interface for evicting pods
type PodEvictor interface {
	EvictPod(pod *v1.Pod) error
}

type PodEvictorImpl struct {
	cli client.WaspClient
}

func NewPodEvictorImpl(cli client.WaspClient) *PodEvictorImpl {
	return &PodEvictorImpl{
		cli: cli,
	}
}

func (pe *PodEvictorImpl) EvictPod(pod *v1.Pod) error {
	// Remove finalizers if any
	if len(pod.ObjectMeta.Finalizers) > 0 {
		pod.ObjectMeta.Finalizers = []string{}
		_, err := pe.cli.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: new(int64),
	}
	*deleteOptions.GracePeriodSeconds = 0
	err := pe.cli.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, deleteOptions)
	if err != nil {
		log.Log.Infof(err.Error())
	}

	/* //todo: Do we need this?
	for _, containerStatus := range pod.Status.ContainerStatuses {
		containerID := containerStatus.ContainerID
		if containerID != "" {
			// Remove the 'docker://' or 'cri-o://' prefix from the container ID
			id := strings.TrimPrefix(containerID, "docker://")
			id = strings.TrimPrefix(id, "cri-o://")

			cmd := exec.Command("crictl", "rm", id)
			err := cmd.Run()
			if err != nil {
				log.Log.Infof(err.Error())
			}
		}
	}
	*/

	return nil
}
