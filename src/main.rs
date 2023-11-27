use lib::*;

fn main() {
    env_logger::init();

    println!("Hello, world!");
    let fs = FS::new();
    let mut wasp = WaspAgent::new(fs);
    wasp.configure_node_swap();
}
