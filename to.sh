IMG_REPO=quay.io/fdeutsch/wasp-operator-prototype

build() {
  podman -r build -t $IMG_REPO -f Containerfile .
}

push() {
  podman -r push $IMG_REPO
}

_oc() { echo "$ oc $@" ; oc $@ ; }
qoc() { oc $@ > /dev/null 2>&1; }

apply() {
  _oc apply -f manifests/ds.yaml
  _oc apply -f manifests/kubelet-confiuration.yaml
}

patch() {
  _oc patch --type=merge  -f manifests/prep-hco.yaml --patch-file manifests/prep-hco.yaml || :
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
  _oc delete -f manifests/kubelet-confiuration.yaml
}

eval "$@"
