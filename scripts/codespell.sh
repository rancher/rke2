#!/bin/bash
set -e

# Ignore vendor folder and check file names as well
# Note: ignore ".te#" in https://github.com/rancher/rke2/blob/eb79cc8/docs/security/selinux.md#L13,L17-L19
codespell --skip=.git,./vendor --check-filenames --ignore-regex=.te#

res=`echo $?`
if [ "$res" = "0" ]; then
    echo "Codespell found no problems"
else
    echo "Codespell found one or more problems"
fi

exit $res
