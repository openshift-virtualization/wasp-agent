#!/usr/bin/bash

c() { echo "# $@" ; }
x() { echo "\$ $@" ; eval "$@" ; }

CACHE_DIR=${CACHE_DIR:-/tmp/pod-cache.d}

kubectl get pods --watch --all-namespaces \
  -o jsonpath='{"NAMESPACE="}{.metadata.namespace}{" NAME="}{.metadata.name}{" PHASE="}{.status.phase}{"\n"}' \
  | while read EVARS;
    do
      export $EVARS
      [[ -z "$NAMESPACE" ]] && continue

      CACHE_FILE="$CACHE_DIR/$NAMESPACE+$NAME.json"
      if [[ "$PHASE" == "Running" ]]; then
        c "Caching pod '$NAMESPACE/$NAME' in $CACHE_FILE"
        x "kubectl get pod -o json -n $NAMESPACE $NAME > $CACHE_FILE"
      else
        [[ -f "$CACHE_FILE" ]] || continue

        c "Un-Caching pod '$NAMESPACE/$NAME' in $CACHE_FILE"
        x rm $CACHE_FILE
      fi
    done
