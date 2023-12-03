use log::{debug,trace};

use std;
use std::collections::{HashMap};
use std::path::{Path, PathBuf};
use wax::Glob;

use k8s_openapi::api::core::v1::Pod;
use k8s_openapi::apimachinery::pkg::api::resource::Quantity;

// For easier mocking
pub struct FS {
    mock: Option<HashMap<String, String>>
}
impl FS {
    pub fn new() -> FS {
        FS {
            mock: None
        }
    }
    pub fn new_mock() -> FS {
        debug!("Mock mode");
        FS {
            mock: Some(HashMap::new())
        }
    }
    /*fn read_to_string(&self, path: &str) -> String {
        debug!("Read {}", path);
        if let Some(vals) = &self.mock {
            vals[path].clone()
        } else {
            std::fs::read_to_string(path).unwrap()
        }
    }*/
    fn exists(&self, path: &str) -> bool {
        if let Some(vals) = &self.mock {
            vals.contains_key(path)
        } else {
            std::fs::metadata(path).ok().is_some()
        }
    }
    fn write_string(&mut self, path: &str, val: &str) {
        if let Some(vals) = &mut self.mock {
            vals.insert(String::from(path), String::from(val));
        } else {
            let _ = std::fs::write(path, val);
        }
    }

    fn cg(&mut self, cg_path: &PathBuf) -> Cgroup {
        Cgroup::open(self, cg_path)
    }

    fn cg_find(&mut self, cg_pat: &str) -> Option<Cgroup> {
        Cgroup::find(self, cg_pat)
    }
}

static CGROUP_BASE: &str = "/sys/fs/cgroup";
struct Cgroup<'a> {
    fs: &'a mut FS,
    path: PathBuf
}
impl<'a> Cgroup<'a> {
    fn open(fs: &'a mut FS, cg_path: &Path) -> Cgroup<'a> {
        let mut pb = PathBuf::from(CGROUP_BASE);
        pb.push(cg_path);
        Cgroup {
            fs: fs,
            path: pb
        }
    }
    fn find(fs: &'a mut FS, pattern: &str) -> Option<Cgroup<'a>> {
        let glob = Glob::new(pattern).unwrap();
        let mut cg = None;
        for entry in glob.walk(CGROUP_BASE) {
            let pb = entry.unwrap();
            cg = Some(Cgroup {
                fs: fs,
                path: pb.into_path()
            });
            break;
        }
        cg
    }
    fn full_path(&self, interface: &str) -> String {
        let mut pb = self.path.clone();
        pb.push(interface.strip_prefix("/").unwrap_or(interface));
        let p = pb.into_os_string().into_string().unwrap();
        p
    }
    fn interface<'b>(&'b mut self, interface_name: &'b str) -> Option<CgInterface> where 'b: 'a {
        let p = self.full_path(interface_name);
        trace!("Interface {}?", p);
        if self.fs.exists(&p) {
            Some(CgInterface::open(self, interface_name))
        } else {
            None
        }
    }
}

struct CgInterface<'a> {
    cg: &'a mut Cgroup<'a>,
    interface_name: &'a str
}
impl<'a> CgInterface<'a> {
    fn open(cg: &'a mut Cgroup<'a>, interface_name: &'a str) -> CgInterface<'a> {
        CgInterface {
            cg,
            interface_name
        }
    }
    fn set(&mut self, val: &str) {
        let full_path = self.cg.full_path(self.interface_name);
        println!("Configuring '{}' to {}", full_path, val);
        self.cg.fs.write_string(&full_path, val);
    }
}

type SwapQuantity = String;
type ContainerId = String;
const SWAP_RESOURCE_NAME: &str = "node.kubevirt.io/swap";

pub struct WaspAgent {
    fs: FS
}
impl WaspAgent {
    pub fn new(fs: FS) -> WaspAgent {
        WaspAgent {
            fs: fs
        }
    }

    pub fn handle_pod(&mut self, pod: &Pod) {
        self.set_container_swap_according_to_pod_swap_resource(pod);
    }

    fn set_container_swap_according_to_pod_swap_resource(&mut self, pod: &Pod) {
        let (cid, swap_quantity) = self.container_id_and_swap_from_pod(pod);

        let pattern = format!("kubepods.slice/*.slice/*.slice/crio-{}.scope", cid);
        let mut cg = self.fs.cg_find(&pattern).unwrap();

        let swap_request_normalized = swap_quantity.0;  //FIXME convert kube to cgroup

        cg.interface("memory.swap.max")
            .unwrap()
            .set(&swap_request_normalized);
    }

    /// Loop containers of pod to find the container with swap resource
    fn container_id_and_swap_from_pod(&self, pod: &Pod) -> (ContainerId, Quantity) {
        let _p = pod;
        let (ref c_name, Some(swap_quantity)) = pod.spec.as_ref().unwrap().containers
            .iter()
            .filter(|c| c.resources.is_some())
            .filter(|c| c.resources.as_ref().unwrap().limits.is_some())
            .filter(|c| c.resources.as_ref().unwrap().limits.as_ref().unwrap().contains_key("memory"))
            .filter(|c| c.resources.as_ref().unwrap().limits.as_ref().unwrap().contains_key(SWAP_RESOURCE_NAME))
            .map(|c| (c.name.clone(),
                      c.resources.as_ref().unwrap().limits.as_ref().unwrap().get(SWAP_RESOURCE_NAME)))
            .collect::<Vec<_>>()[0] else { todo!() };

        let c_id: String = pod.status.as_ref().unwrap().container_statuses.as_ref().unwrap()
            .iter()
            .filter(|c| &c.name == c_name)
            .map(|c| c.container_id.as_ref().unwrap().clone())
            .collect::<Vec<_>>()[0].clone();

        (c_id, swap_quantity.clone())
    }
}


#[cfg(test)]
mod tests {
    use crate::*;
 #[test]
 fn good_walk() {
    let _ = env_logger::builder().is_test(true).try_init();

    let mut fs = FS::new_mock();
    fs.write_string(&"/proc/6553/cgroup",
               &"0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podb312a30a_d345_42c8_a659_32a04d5fd529.slice/crio-0ae1424a0e6b723f775c60e8aea2b45455ed1bbff25492097f2cabe086ba7990.scope");
    fs.write_string(&"/sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podb312a30a_d345_42c8_a659_32a04d5fd529.slice/crio-0ae1424a0e6b723f775c60e8aea2b45455ed1bbff25492097f2cabe086ba7990.scope/memory.swap.max",
               &"0");

    fs.write_string(&"/proc/441/cgroup",
               &"0::/system.slice/systemd-logind.service");
    fs.write_string(&"/sys/fs/cgroup/system.slice/systemd-logind.service/memory.swap.max",
                    &"0");
    let wasp = WaspAgent {
        fs: fs,
    };
    println!("{:?}", wasp.kube_container_cgroups());
    assert_eq!(wasp.kube_container_cgroups().len(), 1);
    assert_eq!(wasp.non_kube_container_cgroups().len(), 1);
 }
}

// curl --header "Content-Type: application/json-patch+json"   --request PATCH   --data '[{"op": "add", "path": "/status/capacity/node.kubevirt.io~1swap", "value": "42"}]'   http://localhost:8001/api/v1/nodes/ci-ln-2bzmr0k-1d09d-xqsxc-worker-centralus3-g266v/status
