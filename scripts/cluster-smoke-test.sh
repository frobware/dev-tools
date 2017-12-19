#!/bin/bash

set -xeuo pipefail

PROJECT=sjenning-jenkins

if [ -z "$1" ]; then
	echo "required cluster param not provided"
	echo "usage: $0 cluster"
    exit 1
fi
APISERVER="https://api.${1}.openshift.com:443"

cleanup() {
	echo "starting cleanup..."
	oc describe pod
	oc get build ruby-ex-1
	oc get dc ruby-ex
	oc delete project $PROJECT
    echo "APISERVER $APISERVER"
}

failure() {
	cleanup
	exit 1
}

reset_project() {
	oc delete project $PROJECT
	wait_for "project_deleted $PROJECT"
}

project_deleted() {
	! oc get project $1 &>/dev/null
}

build_completed() {
	[ "$(oc get build $1 -o json | jq .status.phase)" == "\"Complete\"" ]
}

deployment_completed() {
	[ $(oc get dc $1 -o json | jq .status.readyReplicas) == 1 ]
}

wait_for() {
	for i in $(seq 19 -1 1); do
		if $1; then
			return 0
	        fi
		echo "Retry in 30s for $i more attempt(s)"
		sleep 30
	done
	return 1
}

if [ ! -e oc ]; then
	wget https://github.com/openshift/origin/releases/download/v3.7.0-rc.0/openshift-origin-client-tools-v3.7.0-rc.0-e92d5c5-linux-64bit.tar.gz
    tar xf openshift-origin-client-tools-v3.7.0-rc.0-e92d5c5-linux-64bit.tar.gz
    pushd openshift-origin-client-tools-v3.7.0-rc.0-e92d5c5-linux-64bit
    cp oc ../.
    popd
    rm -rf openshift-origin-client-tools-v3.7.0-rc.0-e92d5c5-linux-64bit
fi
export PATH=$PWD:$PATH
oc login $APISERVER --token=$TOKEN
! oc get project $PROJECT &>/dev/null || reset_project $PROJECT || failure "Project deletion failed"
oc new-project $PROJECT
oc new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git
date
sleep 10
oc get pods -o wide
wait_for "build_completed ruby-ex-1" || failure "Build failed"
wait_for "deployment_completed ruby-ex" || failure "Deployment failed"
echo "Application deployed successfully"
cleanup
