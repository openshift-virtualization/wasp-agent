package cadvisor

const (
	ContainerRuntimeEndpoint = "unix://var/lib/crio.sock"
	RootDirectory            = "/tmp/var/lib/kubelet"
)

type cAdvisorConfig struct {
	ImageFsInfoProvider           ImageFsInfoProvider
	RootPath                      string
	CgroupRoots                   []string
	UsingLegacyStats              bool
	LocalStorageCapacityIsolation bool
}

func NewCAdvisorConfigForCRIO() *cAdvisorConfig {
	return &cAdvisorConfig{
		ImageFsInfoProvider:           NewImageFsInfoProvider(ContainerRuntimeEndpoint),
		RootPath:                      RootDirectory,
		CgroupRoots:                   []string{"/kubepods.slice", "/system.slice"},
		UsingLegacyStats:              UsingLegacyCadvisorStats(ContainerRuntimeEndpoint),
		LocalStorageCapacityIsolation: false, // we don't need fs stats
	}
}
