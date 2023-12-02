IMG_REPO=quay.io/fdeutsch/wasp-operator-prototype

build() {
	podman -r build -t $IMG_REPO -f - . < images/Dockerfile
}

push() {
	podman -r push $IMG_REPO
}

_oc() { echo "$ oc $@" ; oc $@ ; }
qoc() { oc $@ > /dev/null 2>&1; }

deploy() {
	local NS=wasp
	qoc get project wasp || _oc adm new-project $NS
	_oc project $NS
	qoc get sa -n $NS wasp || {
		_oc create sa -n $NS wasp
		#oc adm policy add-role-to-user -n $NS cluster-admin -z wasp
		_oc adm policy add-cluster-role-to-user cluster-admin -z wasp
		_oc adm policy add-scc-to-user -n $NS privileged -z wasp
	}
	_oc apply -f manifests/ds.yaml -f manifests/fedora.yaml
}

destroy() {
	oc delete -f manifests/ds.yaml -f manifests/fedora.yaml
}

$@
