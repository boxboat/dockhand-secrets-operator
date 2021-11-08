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

## Deprecations
See [crd-specs](/usage/crd-specs) for current full specifications

Note removed in `1.0.0`

### Deprecation `version: v1alpha1 DockhandProfile`
Changes:
* `version: dockhand.boxboat.io/v1alpha1` -> `version: dhs.dockhand.dev/v1alpha2`
* `kind: DockhandProfile` -> `kind: Profile`

### Deprecation `version: v1alpha1 DockhandSecret`
Changes:
* `version: dockhand.boxboat.io/v1alpha1` -> `version: dhs.dockhand.dev/v1alpha2`
* `kind: DockhandSecret` -> `kind: Secret`
* `profile: <profile-name>` -> `profile.name: <profile-name>` `profile.namespace: <profile-namespace>`
    * `profile` field is now an object that contains a `name` and `namespace` field.
    * `v1alpha1` assumed `DockhandProfiles` existed in `dockhand-secrets-operator` namespace and did not support multi-tenant use case. `v1alpha2` allows the operator to operate in a multi-tenant mode or a single tenant mode where `Profiles` can be referenced in any namespace where the `dockhand-secrets-operator` has read access.
  