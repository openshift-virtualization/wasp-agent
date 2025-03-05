package wasp

import (
	"fmt"
	"github.com/openshift-virtualization/wasp-agent/pkg/wasp/config"
	oci_hook_render "github.com/openshift-virtualization/wasp-agent/pkg/wasp/oci-hook-render"
	"k8s.io/klog/v2"
	"os"
)

const (
	hookTemplateFile = "/app/OCI-hook/hookscript.template"
	hookScriptPath   = "/host/opt/oci-hook-swap.sh"
	// CrioConfigPath is the default location for the conf file.
	CrioConfigPath = "/host/etc/crio/crio.conf"
	// CrioConfigDropInPath is the default location for the drop-in config files.
	CrioConfigDropInPath = "/host/etc/crio/crio.conf.d"
)

type crioConfiguration interface {
	GetRuntime() (string, error)
}

type hookRenderer interface {
	Render() error
}

func setOCIHook() error {
	err := setupHookScript()
	if err != nil {
		return err
	}

	err = moveFile("/app/OCI-hook/swap-for-burstable.json", "/host/run/containers/oci/hooks.d/swap-for-burstable.json")
	if err != nil {
		return err
	}

	return nil
}

func setupHookScript() error {
	crioConfig := crioConfiguration(config.New(CrioConfigPath, CrioConfigDropInPath))
	runtime, err := crioConfig.GetRuntime()
	if err != nil {
		return err
	}
	klog.Infof("detected runtime " + runtime)

	renderer := hookRenderer(oci_hook_render.New(hookTemplateFile, hookScriptPath, runtime))
	if err := renderer.Render(); err != nil {
		return err
	}

	err = os.Chmod(hookScriptPath, 0755)
	if err != nil {
		return fmt.Errorf("Couldn't set file permissions: %v", err)
	}

	return nil
}
