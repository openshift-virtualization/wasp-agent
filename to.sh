IMG_REPO=quay.io/fdeutsch/wasp-operator-prototype

build() {
	podman -r build -t $IMG_REPO -f - . < images/Dockerfile
}

push() {
	podman -r push $IMG_REPO
}

deploy() {
	oc admin oc adm new-project wasp
	kubectl apply -f manifests/sa.yaml
	kubectl apply -f manifests/ds.yaml
	oc adm policy add-scc-to-user privileged -z wasp
}

destroy() {
	kubectl delete -f manifests/ds.yaml
}

$@
