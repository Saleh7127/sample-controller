#!/bin/bash

set -x

vendor/k8s.io/code-generator/generate-groups.sh all \
  github.com/Saleh7127/sample-controller/pkg/client \
  github.com/Saleh7127/sample-controller/pkg/apis \
  saleh.dev:v1alpha1 \
  --go-header-file /home/user/go/src/github.com/Saleh7127/sample-controller/hack/boilerplate.go.txt