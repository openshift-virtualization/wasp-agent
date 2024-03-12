#!/usr/bin/bash

c() { echo "# $@" ; }
x() { echo "\$ $@" ; eval "$@" ; }

CACHE_DIR=${CACHE_DIR:-/tmp/pod-cache.d}

kubectl get pods --watch --all-namespaces --field-selector spec.nodeName=${NODE_NAME} \
  -o jsonpath='{"NAMESPACE="}{.metadata.namespace}{" NAME="}{.metadata.name}{" TS="}{metadata.deletionTimestamp}{"\n"}' \
  | while read EVARS;
    do
      export $EVARS
      [[ -z "$NAMESPACE" ]] && continue

      CACHE_FILE="$CACHE_DIR/$NAMESPACE+$NAME.json"
      [[ -f "$CACHE_FILE" ]] && [[ ! -z "$TS" ]] && uncache $NAMESPACE $NAME $CACHE_FILE
      [[ -f "$CACHE_FILE" ]] && continue 

      c "Caching pod '$NAMESPACE/$NAME' in $CACHE_FILE"
      x "kubectl get pod -o json -n $NAMESPACE $NAME > $CACHE_FILE"
    done

function uncache() {
  local namespace="$1"
  local name="$2"
  local cache_file="$3"
  c "Un-Caching pod '$NAMESPACE/$NAME' in $CACHE_FILE"
  x rm $CACHE_FILE
}
