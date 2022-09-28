#!/bin/bash

set -x
set -eu

DEBUG="${DEBUG:-false}"

env | egrep '^(AWS).*\=.+' | sort > .env

if [ "false" != "${DEBUG}" ]; then
    cat .env
fi