use lib::*;

fn main() {
    env_logger::init();

    let fs = FS::new();
    let mut _wasp = WaspAgent::new(fs);
}
