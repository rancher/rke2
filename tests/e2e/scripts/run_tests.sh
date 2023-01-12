#!/bin/bash
nodeOS=${1:-"generic/ubuntu2004"}
servercount=${2:-3}
agentcount=${3:-1}
db=${4:-"etcd"}
hardened=${5:-""}
rke2_version=${rke2_version}
rke2_channel=${rke2_channel:-"commit"}

E2E_EXTERNAL_DB=$db && export E2E_EXTERNAL_DB
E2E_REGISTRY=true && export E2E_REGISTRY

cd
cd rke2 && git pull --rebase origin master
/usr/local/go/bin/go mod tidy

cd tests/e2e
OS=$(echo "$nodeOS"|cut -d'/' -f2)
echo "$OS"

# create directory to store reports if it does not exists
if [ ! -d createreport ]
then
	mkdir createreport
fi

count=0
run_tests(){
	count=$(( count + 1 ))
	vagrant global-status | awk '/running/'|cut -c1-7| xargs -r -d '\n' -n 1 -- vagrant destroy -f

	echo 'RUNNING DUALSTACK VALIDATION TEST'
	E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v dualstack/dualstack_test.go -nodeOS="$nodeOS" -serverCount=1 -agentCount=1  -timeout=30m -json -ci |tee  createreport/rke2_"$OS".log

	echo 'RUNNING CLUSTER VALIDATION TEST'
	E2E_REGISTRY=true E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v validatecluster/validatecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount))  -timeout=30m -json -ci |tee -a createreport/rke2_"$OS".log


	echo 'RUNNING MIXEDOS TEST'
	/usr/local/go/bin/go test -v mixedos/mixedos_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -timeout=1h -json -ci |tee -a  createreport/rke2_"$OS".log

	echo 'RUNNING SPLIT SERVER VALIDATION TEST'
	E2E_HARDENED="$hardened" /usr/local/go/bin/go test -v splitserver/splitserver_test.go -nodeOS="$nodeOS" -timeout=30m -json -ci |tee -a createreport/rke2_"$OS".log

	E2E_RELEASE_VERSION=$rke2_version && export E2E_RELEASE_VERSION
	E2E_RELEASE_CHANNEL=$rke2_channel && export E2E_RELEASE_CHANNEL

	echo 'RUNNING CLUSTER UPGRADE TEST'
	E2E_REGISTRY=true /usr/local/go/bin/go test -v upgradecluster/upgradecluster_test.go -nodeOS="$nodeOS" -serverCount=$((servercount)) -agentCount=$((agentcount)) -timeout=1h -json -ci |tee -a createreport/rke2_"$OS".log
}

ls createreport/rke2_"$OS".log 2>/dev/null && rm createreport/rke2_"$OS".log
run_tests

# re-run test if first run fails and keep record of repeatedly failed test to debug
while [ -f createreport/rke2_"$OS".log ] && grep -w ":fail" createreport/rke2_"$OS".log && [ $count -le 2 ]
do
        cp createreport/rke2_"$OS".log createreport/rke2_"$OS"_"$count".log
        run_tests
done
