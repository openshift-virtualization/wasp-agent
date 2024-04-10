#!/usr/bin/bash

source common.sh

NODE_CAPACITY_BYTES=$WASP_DIR/node_capacity_bytes
SWAP_CAPACITY_BYTES=$WASP_DIR/swap_capacity_bytes

uncache() {
  local namespace="$1"
  local name="$2"
  local cache_file="$3"
  c "Un-Caching pod '$namespace/$name' in $cache_file"
  x rm $cache_file
}

calculate_swap() {
  local mem_req="$1"
  local mem_req_bytes=0
  node_capacity=$(cat $NODE_CAPACITY_BYTES)
  swap_capacity=$(cat $SWAP_CAPACITY_BYTES)

  case $mem_req in
  *[i])
    mem_req_bytes=$(numfmt --from=iec-i $mem_req)
    ;;
  *)
    mem_req_bytes=$(numfmt --from=iec $mem_req)
    ;;
  esac
  mem_req_bytes=$(echo "scale=1; ( $swap_capacity / $node_capacity ) * $mem_req_bytes" | bc -l )
  echo ${mem_req_bytes%.*}
}

process_container_data(){
  local data="$1"
  local podname="$2"
  local podnamespace="$3"
  local podfile="$4"

  export container_name=$(echo $data | cut -d',' -f 1)
  mem_req=$(echo $data | cut -d',' -f 2)

  pod_id=$(cat $podfile | jq '.metadata.uid | split("-")| join("_")' | tr -d "'\"")
  pod_config_hash=$(cat $podfile | jq '.metadata.annotations["kubernetes.io/config.hash"]' | tr -d "'\"")
  container_id=$(cat $podfile | jq '.status.containerStatuses[]| select(.name==env.container_name) | .containerID | split("//")[1]' | tr -d "'\"")
  cgroups_base_path="/sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/"
  slice_prefix="kubepods-burstable-pod"
  slice_suffix=".slice"
  scope_prefix="crio-"
  scope_suffix=".scope"
  cgroups_path="$cgroups_base_path/$slice_prefix$pod_id$slice_suffix/$scope_prefix$container_id$scope_suffix"
  if [ ! -d $cgroups_path ]; then
    cgroups_path="$cgroups_base_path/$slice_prefix$pod_config_hash$slice_suffix/$scope_prefix$container_id$scope_suffix"
  fi

  c "Processing data for container $container_name of pod $podname in namespace $podnamespace"
  c "Pod ID $pod_id container ID $container_id"

  echo $(calculate_swap $mem_req) > $cgroups_path/memory.swap.max
}

handle_pod(){
  local filename="$(basename $1)"

  pod_namespace=$(echo ${filename%.*} | cut -d'+' -f 1)
  pod_name=$(echo ${filename%.*} | cut -d'+' -f 2)

  # create a list of (container name, memory request) tuples, pass each tuple item to container process function
  # memory request is optional value in pods so take 100M default as it appears in kubelet

  podfile="$POD_CACHE_DIR/$filename"
  cat $podfile | jq '.spec.containers[] | [.name, .resources.requests.memory // "100M" ] | @csv' \
  | while read LINE;
  do
    data=$(echo $LINE | tr -d "'\"")
    process_container_data $data $pod_name $pod_namespace $podfile
  done
}

prepare_node_info() {
  echo "Preparing node info"
  echo $(free -b | grep Mem | awk -F" " '{print $2}') > $NODE_CAPACITY_BYTES
  echo $(free -b | grep Swap | awk -F" " '{print $2}') > $SWAP_CAPACITY_BYTES
}

pod_cache(){
  while true; do
    oc get pods --watch --all-namespaces --field-selector spec.nodeName="${NODE_NAME}" \
      -o jsonpath='{" NAMESPACE="}{.metadata.namespace}{" NAME="}{.metadata.name}{" TS="}{.metadata.deletionTimestamp}{" PHASE="}{.status.phase}{" QOS="}{.status.qosClass}{"\n"}' \
      | while read EVARS;
        do
          export $EVARS
          [[ -z "$NAMESPACE" ]] && continue
          [[ "$QOS" != "Burstable" ]] && continue

          CACHE_FILE="$POD_CACHE_DIR/$NAMESPACE+$NAME.json"

          [[ -f "$CACHE_FILE" ]] && [[ ! -z "$TS" ]] && uncache $NAMESPACE $NAME $CACHE_FILE && continue
          [[ ! -z "$TS" ]] && continue

          [[ -f "$CACHE_FILE" ]] && [[ "$PHASE" == "Terminating" ]] && uncache $NAMESPACE $NAME $CACHE_FILE && continue
          [[ "$PHASE" == "Terminating" ]] && continue

          [[ -f "$CACHE_FILE" ]] && [[ "$PHASE" == "Completed" ]] && uncache $NAMESPACE $NAME $CACHE_FILE && continue
          [[ "$PHASE" == "Completed" ]] && continue

          [[ "$PHASE" != "Running" ]] && continue

          c "Caching pod '$NAMESPACE/$NAME' in $CACHE_FILE"
          x "oc get pod -o json -n $NAMESPACE $NAME > $CACHE_FILE"
          handle_pod $CACHE_FILE
        done
  done

}
