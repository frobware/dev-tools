#/bin/bash

set -xeuo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
. $DIR/prep-kube.sh
cd $GOPATH/src/k8s.io/kubernetes
make test-e2e-node TEST_ARGS='--report-dir=/tmp/artifacts --kubelet-flags="--cgroup-driver=systemd"'
