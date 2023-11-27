use log::{debug,trace};

use std::fs;
use std::vec::Vec;
use std::collections::{HashMap,BTreeSet};
use std::path::PathBuf;

use wax::{Glob,Pattern};

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
    fn find(&self, path: &str, pattern: &str) -> Vec<String> {
        debug!("In {} find {}", path, pattern);
        if let Some(vals) = &self.mock {
            vals.keys()
                .filter(|e| e.starts_with(path))
                .filter(|e| Glob::new(pattern).unwrap().is_match(e.as_str()))
                .map(String::from)
                .collect()
        } else {
            let glob = Glob::new(pattern).unwrap();
            glob.walk(path)
                .into_iter()
                .filter(|e| e.as_ref().ok().is_some())
                .map(|e| String::from(e.unwrap().path().to_str().unwrap()))
                .collect()
        }
    }
    fn read_to_string(&self, path: &str) -> String {
        debug!("Read {}", path);
        if let Some(vals) = &self.mock {
            vals[path].clone()
        } else {
            fs::read_to_string(path).unwrap()
        }
    }
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
    fn cg_full_path(&self, cg: &str, key: &str) -> String {
        let mut pb = PathBuf::from("/sys/fs/cgroup");
        pb.push(cg.strip_prefix("/").unwrap_or(cg));
        pb.push(key.strip_prefix("/").unwrap_or(key));
        let p = pb.into_os_string().into_string().unwrap();
        debug!("pb {}", p);
        p
    }
    fn cg_has_interface(&self, cg: &str, key: &str) -> bool {
        let p = self.cg_full_path(&cg, key);
        debug!("Has interface {}?", p);
        self.exists(&p)
    }
    fn cg_set(&mut self, cg: &str, key: &str, val: &str) {
        let full_path = self.cg_full_path(cg, key);
        println!("Configuring '{}' to {}", full_path, val);
        self.write_string(&full_path, val);
    }

    fn cri_get_container_id_from_cgroup(cg: &str) -> String {
        String::from("")
    }
}


pub struct WaspAgent {
    fs: FS,
    crio: CrioHacker
}
impl WaspAgent {
    pub fn new(fs: FS) -> WaspAgent {
        WaspAgent {
            fs: fs,
            crio: CrioHacker::new()
        }
    }

    fn cgroups_of_procs(&self) -> Vec<String> {
        debug!("Finding cgroups of proccesses");
        let mut cgroups = Vec::new();
        for entry in self.fs.find("/proc", "**/cgroup") {
            debug!("Has cgroup? {}", entry);
            //if entry.contains(".slice") { continue ; }
            let raw = self.fs.read_to_string(&entry);
            debug!("Got: {}", raw);
            let cg = String::from(raw[3..].trim());
            if self.fs.cg_has_interface(&cg, "memory.swap.max") {
                debug!("Has swap, consider it");
                cgroups.push(cg);
            } else {
                debug!("Has no swap, drop it");
            }
        }
        BTreeSet::from_iter(cgroups.into_iter())
            .into_iter()
            .collect()
    }

    pub fn kube_container_cgroups(&self) -> Vec<String> {
        debug!("Finding kube container cgroups");
        self.cgroups_of_procs()
            .into_iter()
            .map(|pth| {trace!("F {}", pth); pth })
            .filter(|pth| pth.contains("kubepods.slice"))
            .filter(|pth| !pth.contains("conmon"))
            .collect()
    }

    fn non_kube_container_cgroups(&self) -> Vec<String> {
        debug!("Finding non-kube cgroups");
        let all_cgroups = self.cgroups_of_procs().into_iter();
        let kube_cgroups = self.kube_container_cgroups().into_iter();

        BTreeSet::from_iter(all_cgroups)
            .difference(&BTreeSet::from_iter(kube_cgroups))
            .map(String::from)
            .collect()
    }

    fn configure_container_swap(&mut self, cg: &str) {
        let val = "0"; //self.get_swap_resource_from_container_cgroup(cg);
        self.fs.cg_set(cg, "memory.swap.max", val);
    }

    fn get_swap_resource_from_container_cgroup(&self, path: &str) -> &str {
        let _res_name = "node.kubevirt.io/swap";
        let _ = path;
        "0"
    }

    pub fn configure_node_swap(&mut self) {
        debug!("hi");
        self.non_kube_container_cgroups()
            .iter()
            .for_each(|cg| self.fs.cg_set(cg, "memory.swap.max", "0"));
        self.kube_container_cgroups()
            .iter()
            .for_each(|cg| self.configure_container_swap(cg));
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
