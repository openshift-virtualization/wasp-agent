#!/bin/bash -e

source ./hack/build/config.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source cluster-sync/install.sh

echo "Cleaning up ..."

OPERATOR_MANIFEST=./_out/manifests/release/wasp.yaml
LABELS=("wasp.io")
NAMESPACES=(default kube-system "${WASP_NAMESPACE}")


delete_wasp

# Everything should be deleted by now, but just to be sure
for n in ${NAMESPACES[@]}; do
  for label in ${LABELS[@]}; do
    _kubectl -n ${n} delete deployment -l ${label} >/dev/null
    _kubectl -n ${n} delete services -l ${label} >/dev/null
    _kubectl -n ${n} delete secrets -l ${label} >/dev/null
    _kubectl -n ${n} delete configmaps -l ${label} >/dev/null
    _kubectl -n ${n} delete pods -l ${label} >/dev/null
    _kubectl -n ${n} delete rolebinding -l ${label} >/dev/null
    _kubectl -n ${n} delete roles -l ${label} >/dev/null
    _kubectl -n ${n} delete serviceaccounts -l ${label} >/dev/null
  done
done

for label in ${LABELS[@]}; do
    _kubectl delete pv -l ${label} >/dev/null
    _kubectl delete clusterrolebinding -l ${label} >/dev/null
    _kubectl delete clusterroles -l ${label} >/dev/null
    _kubectl delete customresourcedefinitions -l ${label} >/dev/null
done

if [ "${WASP_CLEAN}" == "all" ] && [ -n "$(_kubectl get ns | grep "wasp ")" ]; then
    echo "Clean wasp namespace"
    _kubectl delete ns ${WASP_NAMESPACE}

    start_time=0
    sample=10
    timeout=120
    echo "Waiting for wasp namespace to disappear ..."
    while [ -n "$(_kubectl get ns | grep "${WASP_NAMESPACE} ")" ]; do
        sleep $sample
        start_time=$((current_time + sample))
        if [[ $current_time -gt $timeout ]]; then
            exit 1
        fi
    done
fi
sleep 2
echo "Done"
