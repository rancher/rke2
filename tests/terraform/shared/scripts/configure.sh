#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"

env | egrep '^(AWS|RKE2).*\=.+' | sort > .env

if [ "false" != "${DEBUG}" ]; then
    cat .env
fi