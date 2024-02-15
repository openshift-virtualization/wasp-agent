IMG_REPO=quay.io/fdeutsch/wasp-operator-prototype

build() {
  podman -r build -t $IMG_REPO -f Containerfile .
}

push() {
  podman -r push $IMG_REPO
}

i() { echo "i: $@"; }
x() { echo "\$ $@" ; eval "$@" ; }
die() { echo "err: $@" ; exit 1; }
_oc() { echo "$ oc $@" ; oc $@ ; }
qoc() { oc $@ > /dev/null 2>&1; }

apply() {
  _oc apply -f manifests/ds.yaml
  _oc apply -f manifests/kubelet-configuration-with-swap.yaml
  _oc apply -f manifests/machineconfig-add-swap.yaml
  -oc apply -f manifests/prometheus-rules.yaml
  qoc get namespace openshift-cnv && _oc patch --type=merge  -f manifests/prep-hco.yaml --patch-file manifests/prep-hco.yaml || i "No CNV, No HCO patch"
}


deploy() {
  local NS=wasp
  local SA=$NS
  qoc get project $NS || _oc adm new-project $NS
  _oc project $NS
  qoc get sa -n $NS $SA || {
    _oc create sa -n $NS $SA
    _oc adm policy add-cluster-role-to-user cluster-admin -z $SA
    _oc adm policy add-scc-to-user -n $NS privileged -z $SA
  }
  apply
}

destroy() {
  _oc delete -f manifests/ds.yaml
  _oc delete -f manifests/machineconfig-add-swap.yaml
  _oc delete -f manifests/kubelet-configuration-with-swap.yaml
  _oc delete -f manifests/prometheus-rules.yaml
  qoc get namespace openshift-cnv && sed 's#"add"#"remove"#' manifests/prep-hco.yaml | _oc patch --type=merge  -f manifests/prep-hco.yaml --patch - || i "No CNV, No HCO patch"
}


wait_for_mcp() {
  x "oc wait mcp worker --for condition=Updated=False --timeout=10s"
  x "oc wait mcp worker --for condition=Updated=True --timeout=15m"
}

check_nodes() {
  oc get nodes -l node-role.kubernetes.io/worker -ocustom-columns=NAME:.metadata.name --no-headers \
    | while read W ; do oc debug node/$W -- sh -c "free -m" 2>&1 | grep -E "^(Starting|Swap)" ; done
}

usage() {
  grep -E -o "^.*\(\)" $0
}

eval "${@:-usage}"
