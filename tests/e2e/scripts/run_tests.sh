#!/bin/bash
# Usage: ./run_tests.sh
# This script runs all the rke2 e2e tests and generates a report with the log
# The generated log is placed in createreport/rke2_${date}.log
#
# This script must be run inside the rke2 directory where the tests exist
#
# Example:
#   To run the script with default settings:
#     ./run_tests.sh
#
set -x

# tests to run
tests=("ciliumnokp" "dnscache" "dualstack" "mixedos" "mixedosbgp" "multus" "secretsencryption" "splitserver" "upgradecluster" "validatecluster")
date=$(date +%Y%m%d)

E2E_REGISTRY=true && export E2E_REGISTRY

git pull --rebase origin master
/usr/local/go/bin/go mod tidy
cd tests/e2e

# create directory to store reports if it does not exists
if [ ! -d createreport ]
then
	mkdir createreport
fi

# Remove VMs which are in invalid state
vagrant global-status --prune

count=0
run_tests(){

    count=$(( count + 1 ))

    for i in ${!tests[@]}; do
	pushd ${tests[$i]}
	vagrant destroy -f

        echo "RUNNING ${tests[$i]} TEST"
        /usr/local/go/bin/go test -v ${tests[$i]}_test.go -timeout=1h -json -ci |tee -a ../createreport/rke2_${date}.log
        
	popd
    done
}

ls createreport/rke2_${date}.log 2>/dev/null && rm createreport/rke2_${date}.log
run_tests

# re-run test if first run fails and keep record of repeatedly failed test to debug
while [ -f createreport/rke2_${date}.log ] && grep -w " FAIL:" createreport/rke2_${date}.log && [ $count -le 2 ]
do
        cp createreport/rke2_${date}.log createreport/rke2_${date}_${count}.log
        run_tests
done

# Generate report and upload to s3 bucket
cd createreport && /usr/local/go/bin/go run -v report-template-bindata.go generate_report.go -f rke2_${date}.log
