FSROOT=${FSROOT:-/host}
WASP_DIR="$FSROOT/tmp/wasp"
PID_FILE="$WASP_DIR/pod_cache"

POD_CACHE_DIR=${POD_CACHE_DIR:-"$WASP_DIR/pod-cache.d"}

c() { echo "# $@" ; }
x() { echo "\$ $@" ; eval "$@" ; }
