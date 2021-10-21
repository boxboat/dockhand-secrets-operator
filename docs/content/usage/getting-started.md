---
title: Getting Started
weight: -80
---

This page tells you how to get started with the dockhand-secrets-operator, including installation and basic usage.

<!--more-->

{{< toc >}}

## Install

`dockhand-secrets-operator` is installed via 2 Helm charts:
- dockhand-secrets-operator-crd
- dockhand-secrets-operator

These charts can be found in the [dockhand-charts](https://github.com/boxboat/dockhand-charts) repository. 

```Shell
# install dockhand-secrets-operator-crd
helm repo add dockhand https://dockhand-charts.storage.googleapis.com
helm repo update
helm install --namespace dockhand-secrets-operator dockhand/dockhand-secrets-operator-crd

# install dockhand-secrets-operator
helm repo add dockhand https://dockhand-charts.storage.googleapis.com
helm repo update
helm install --namespace dockhand-secrets-operator dockhand/dockhand-secrets-operator
```

## Add Dockhand Profile
Once the operator is installed, you will need to give it access to the Secrets Manager(s) that you want to use on the cluster. This is accomplished by creating a `Profile`. See [core-concepts](/usage/core-concepts) 

## Add Dockhand Secrets
Start adding Dockhand `Secrets` to your deployment manifests! See [core-concepts](/usage/core-concepts) 