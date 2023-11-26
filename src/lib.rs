use std::fs;
use std::vec::Vec;
use std::collections::BTreeSet;
use wax::Glob;

fn cgroups_of_procs() -> Vec<String> {
    let mut cgroups = Vec::new();
    let glob = Glob::new("*/cgroup").unwrap();
    for entry in glob.walk("/proc") {
        if let Some(_f) = entry.ok() {
            if let Some(f) = _f.path().to_str() {
                if f.contains(".slice") { continue ; }
                let raw = fs::read_to_string(f).unwrap();
                let grp_path = format!("/sys/fs/cgroup/{}", &raw[3..].trim());
                let swpmax = String::from(&grp_path) + "/memory.swap.max";
                if std::fs::metadata(&swpmax).ok().is_some() {
                    cgroups.push(grp_path);
                }
            }
        }
    }
    BTreeSet::from_iter(cgroups.into_iter())
        .into_iter()
        .collect()
}

pub fn kube_cgroups() -> Vec<String> {
    cgroups_of_procs()
        .into_iter()
        .filter(|pth| pth.contains("kubepods.slice"))
        .filter(|pth| !pth.contains("conmon"))
        .collect()
}

pub fn non_kube_cgroups() -> Vec<String> {
    BTreeSet::from_iter(cgroups_of_procs().into_iter())
        .difference(&BTreeSet::from_iter(kube_cgroups().into_iter()))
        .map(String::from)
        .collect()
}

fn configure_swap(path: &str, val: &str) {
    std::fs::write(String::from(path) + "/memory.swap.max", val);
}

fn no_swap(path: &str) {
    configure_swap(path, "0");
}

fn with_swap(path: &str, val: &str) {
    configure_swap(path, val);
}
