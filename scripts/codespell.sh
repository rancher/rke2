#!/bin/bash
set -e

# Ignore vendor folder and check file names as well
# Note: ignore ".te#" in https://github.com/rancher/rke2/blob/eb79cc8/docs/security/selinux.md#L13,L17-L19
codespell --skip=.git,./vendor,./MAINTAINERS,go.mod,go.sum --check-filenames --ignore-regex=.te# --ignore-words=.codespellignore

code=$?
if [ $code -ne 0 ]; then
  echo "Error: codespell found one or more problems!"
  exit $code
fi
