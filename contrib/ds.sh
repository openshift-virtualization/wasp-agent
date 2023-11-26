

_allproccgroups() {
cat /proc/*/cgroup \
	| grep "\.slice" \
	| cut -d ":" -f3 \
	| sed "s#^#/sys/fs/cgroup/#" \
	| while read FN ; do [[ -f "$FN/memory.swap.max" ]] && echo $FN ; done
}

cgroups_with_pod() {
	_allproccgroups | grep kubepods.slice | grep -v conmon
}

cgroups_without_pod() {
	_allproccgroups | grep -v kubepods.slice
}


configureNoSwap() { # PATH
	configureSwap $1 0 ; }

configureSwap() { # PATH VAL
	echo "$2 > $1/memory.swap.max" ; }

containerNameFromPath() { # PATH
	 egrep -o "crio-[^.]*" | cut -d "-" -f2 ; }

cgroups_without_pod | while read FN ; do configureNoSwap $FN ; done
cgroups_with_pod | while read FN ; do configureSwap $FN 100M ; done
