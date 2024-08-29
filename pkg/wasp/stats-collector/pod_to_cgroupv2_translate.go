package stats_collector

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"path"
	"strings"
)

const (
	OpenshiftKubeletCgroupName = "/kubepods.slice"
)

type PodToCgroupV2Translate struct {
	Pod *v1.Pod
}

func (p *PodToCgroupV2Translate) PodToCgroupSlice() (string, error) {
	uid := string(p.Pod.UID)
	if len(uid) == 0 || uid == "" {
		return "", fmt.Errorf("Pod UID is empty")
	}

	uid = strings.Replace(uid, "-", "_", -1)
	return fmt.Sprintf("pod%s.slice", uid), nil
}

func (p *PodToCgroupV2Translate) PodQosClassToCgroupPath() (string, error) {
	switch p.Pod.Status.QOSClass {
	case v1.PodQOSGuaranteed:
		return "", nil
	case v1.PodQOSBurstable:
		return "kubepods-burstable", nil
	case v1.PodQOSBestEffort:
		return "kubepods-besteffort", nil
	default:
		return "", fmt.Errorf("cound not determine pod QOS")
	}
}

func (p *PodToCgroupV2Translate) PodAbsCgroupPath() (string, error) {
	podQos, err := p.PodQosClassToCgroupPath()
	if err != nil {
		return "", err
	}

	podCgroup, err := p.PodToCgroupSlice()
	if err != nil {
		return "", err
	}

	result := path.Clean(path.Join(
		OpenshiftKubeletCgroupName,
		fmt.Sprintf("%s.slice", podQos),
		fmt.Sprintf("%s-%s", podQos, podCgroup)))

	return result, nil
}

func (p *PodToCgroupV2Translate) PodContainersToCgroupPath() (map[string]string, error) {
	podPath, err := p.PodAbsCgroupPath()
	if err != nil {
		return nil, err
	}

	containers := make(map[string]string)

	for _, ctr := range p.Pod.Status.ContainerStatuses {
		if ctr.Ready && ctr.State.Running != nil {
			key := ctr.Name
			parts := strings.Split(strings.Trim(ctr.ContainerID, "\""), "://")
			if len(parts) != 2 {
				klog.Warningf("Invalid container ID for %s", key)
				continue
			}
			containerAbsCgroupPath := path.Join(
				podPath,
				fmt.Sprintf("crio-%s.scope", parts[1]))
			containers[key] = containerAbsCgroupPath
		}
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("none of the pod containers are in running state")
	}

	return containers, nil
}
