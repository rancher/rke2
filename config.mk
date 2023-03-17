SHELL := /bin/bash

CONFIG_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
LOCAL_TFVARS_PATH := $(CONFIG_DIR)tests/terraform/modules/config/local.tfvars

ifeq ($(wildcard ${LOCAL_TFVARS_PATH}),)
  RESOURCE_NAME :=
  ACCESS_KEY_LOCAL :=
else
  export RESOURCE_NAME := $(shell sed -n 's/resource_name *= *"\([^"]*\)"/\1/p' ${LOCAL_TFVARS_PATH})
  export ACCESS_KEY_LOCAL := $(shell sed -n 's/access_key_local *= *"\([^"]*\)"/\1/p' ${LOCAL_TFVARS_PATH})
endif

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY