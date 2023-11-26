use lib::*;

fn main() {
    println!("Hello, world!");
    for g in non_kube_cgroups().iter() {
        println!("{}", g);
    }
}
