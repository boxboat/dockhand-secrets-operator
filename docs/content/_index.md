---
title: Dockhand Secrets Operator
description: Secure Kubernetes Secrets Manager Integration
geekdocNav: false
geekdocAlign: center
geekdocAnchor: false
---

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD033 -->

<span class="badge-placeholder">[![Build Status](https://img.shields.io/github/actions/workflow/status/boxboat/dockhand-secrets-operator/docker.yaml?master)](https://github.com/boxboat/dockhand-secrets-operator)</span>
<span class="badge-placeholder">[![GitHub release](https://img.shields.io/github/v/release/boxboat/dockhand-secrets-operator)](https://github.com/boxboat/dockhand-secrets-operator/releases/latest)</span>
<span class="badge-placeholder">[![GitHub contributors](https://img.shields.io/github/contributors/boxboat/dockhand-secrets-operator)](https://github.com/boxboat/dockhand-secrets-operator/graphs/contributors)</span>
<span class="badge-placeholder">[![License: APACHE](https://img.shields.io/github/license/boxboat/dockhand-secrets-operator)](https://github.com/boxboat/dockhand-secrets-operator/blob/main/LICENSE)</span>

<!-- markdownlint-restore -->

Secrets management with full GitOps can be challenging in Kubernetes environments. Often engineers resort to manual secret creation,  injection of secrets through scripts with the CI/CD tool or even worse just committing the secrets directly to git.

The Dockhand Secrets Operator solves that problem by allowing you to make arbitrary secrets in a familiar way with only the secret bits stored in the backend(s) of your choice - AWS Secrets Manager, Azure Key Vault, GCP Secrets Manager or Vault. Secret references can be stored in git with your Helm chart or Kubernetes manifests.

{{< button size="large" relref="usage/getting-started/" >}}Getting Started{{< /button >}}
