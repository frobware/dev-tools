#/bin/bash

FOCUS=
if [ "$1" == "conformance" ]; then
	FOCUS="\[Conformance\]"
fi
echo FOCUS=$FOCUS

set -xeuo pipefail

GOVERSION=1.10.1

cd $HOME
yum install gcc -y
rm -rf /tmp/artifacts/* /tmp/e2e_node* /tmp/run_local* artifacts/
if [ ! -d "go$GOVERSION" ]; then
	curl -OL https://storage.googleapis.com/golang/go$GOVERSION.linux-amd64.tar.gz
	mkdir $PWD/go$GOVERSION
	pushd $PWD/go$GOVERSION
	tar -xzf ../go$GOVERSION.linux-amd64.tar.gz
	popd
fi
GOROOT=$PWD/go$GOVERSION/go
PATH=$GOROOT/bin:$PATH
export GOPATH=$PWD/go
pushd go/src/k8s.io
if [ ! -d kubernetes ]; then
	git clone https://github.com/kubernetes/kubernetes.git
fi
pushd kubernetes
git checkout master
git fetch
git reset --hard origin/master
git clean -fdx
make test-e2e-node TEST_ARGS='--report-dir=/tmp/artifacts --kubelet-flags="--cgroup-driver=systemd"' FOCUS=$FOCUS
