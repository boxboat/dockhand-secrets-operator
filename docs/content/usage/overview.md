---
title: Overview
weight: -100
---

## What is the Dockhand Secrets Operator?

Dockhand Secrets Operator is a Secrets Management Kubernetes Operator.

## Why Dockhand Secrets Operator?
Creation of arbitrary Kubernetes Secrets for your application deployments should be declarative, repeatable, easy, secure and flexible. In an ideal world, your Kubernetes `Secrets` manifests would be stored in your git repository alongside your application deployment manifests - allowing for full GitOps. The obvious problem is then your secrets aren't secret anymore. 

`dockhand-secrets-operator` gives you the next best thing a `CustomResourceDefinition` - Dockhand `Secret` that has feature parity with a Kubernetes `Secret` manifest. The `Secret` feels like a regular `Secret` but provides a familiar Go templating syntax that will allow the operator to make your Kubernets `Secret` during your manifest deployment.

`dockhand-secrets-operator` can also provide automatic rollover of `Deployments`, `DaemonSets` and `StatefulSets` - with the addition of a single `Label`.

## How it works
`dockhand-secrets-operator` monitors the creation and update of Dockhand `Secret`, parses the spec and creates a corresponding Kubernetes `Secrets`. Optionally, with addition of a label on a `Deployment`, `StatefulSet` or `DaemonSet` the operator will also checksum the secret and insert a managed annotation, which will trigger an update in accordance with the update policy on each of those types.

If you wish to have a fully automatic experience, you can enable a `syncInterval` on a per Dockhand `Secret` basis that will instruct the operator to poll your Secrets backend for changes. When a change is detected, the operator will rollout a new `Deployment`, `StatefulSet` or `DaemonSet`.
