SHELL := /bin/bash

TFVARS_PATH := terraform/modules/config/local.tfvars

ifeq ($(wildcard ${TFVARS_PATH}),)
  RESOURCE_NAME :=
else
  export RESOURCE_NAME := $(shell sed -n 's/resource_name *= *"\([^"]*\)"/\1/p' ${TFVARS_PATH})
endif

export ACCESS_KEY_LOCAL
export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY