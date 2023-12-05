```console
# Build
$ bash to.sh build
$ bash to.sh push  # only to my account right now

# Deploy
$ <log into ocp 4.15 or higher>
$ bash to.sh deploy

# Demo
$ oc apply -f manifests/demo.sh

# Destroy
$ bash to.sh destroy
```

It's doing mainly two things
1. `echo /proc/sys/vm/swappiness > 100"
2. For each container with swap resource `echo $SWAP_REQ > memory.swap.max`

Note
- Use pod with limit
