---
title: Core Concepts
weight: -90
---
<!--more-->

{{< toc >}}



## DockhandSecretsProfile
A `DockhandSecretsProfile` can contain one or more secrets backends and provides the `dockhand-secrets-operator` with the information it needs to connect to a Secrets Manager.

### Example: DockhandSecretProfile
```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecretsProfile
metadata:
  name: dockhand-profile
  namespace: dockhand-secrets-operator
awsSecretsManager:
  cacheTTL: 60s
  region: us-east-1
  accessKeyId: <accessKeyId>
  secretAccessKeyRef:
    name: dockhand-profile-secrets
    key: aws-secret-access-key
azureKeyVault:
  cacheTTL: 60s
  keyVault: dockcmd
  tenant: <tenantId>
  clientId: <clientId>
  clientSecretRef:
    name: dockhand-profile-secrets
    key: azure-client-secret
gcpSecretsManager:
  cacheTTL: 60s
  project: myproject
  credentialsFileSecretRef:
    name: dockhand-profile-secrets
    key: gcp-credentials.json
vault:
  cacheTTL: 60s
  addr: http://vault:8200
  tokenRef:
    name: dockhand-profile-secrets
    key: vault-token
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: dockhand-profile-secrets
  namespace: dockhand-secrets-operator
data:
  aws-secret-access-key: <Base64 AWS SECRET KEY>
  vault-token: <Base64 encoded vault token>
  azure-client-secret: <Base64 encoded azure client secret>
  gcp-credentials.json: <Base64 encoded GCP JSON file>
```

## DockhandSecret

`DockhandSecret` is essentially a Go template with alternate delimiters `<< >>` so that you can use it in a Helm chart. The operator is built off [dockcmd](https://github.com/boxboat/dockcmd). Sprig functions are supported and specific versions of secrets are supported through the use of `?version=` on the secret name. For simplicity `?version=latest` will work with all of the backends but specific versions require the value expected by the backend.

Note that GCP and Azure have 2 forms of supported secrets `text` or `json`. The text version will return the entire secret stored in the key/value whereas the json version will interpret the stored value as `json` and allow you retrieve a single `key` in the secret.

The `DockhandSecret` will generate a secret of type `secretSpec.type` in the same namespace specified by `secretSpec.name`. Changes to a `DockhandSecret` will trigger a refresh of the `Secret` managed by that `DockhandSecret`. You can optionally have labels or annotations injected on the `Secret` created by the `DockhandSecret`.

The `dockhandProfile` field allows you to specify different `DockhandSecretsProfiles`, which gives you flexibility to connect to numerous Secrets Managers from the same cluster.

### AWS Secrets Manager
`DockhandSecret` supports retrieval of an AWS Secrets Manager `json` secret using `<< (aws <secret-name> <json-key>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Suppose you have an AWS Secrets Manager Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }`. The following `DockhandSecret` would generate create an `Opaque` `Secret` in the `aws` namespace.
```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: example-aws-dockhand
  namespace: aws
profile: dockhand-profile
secretSpec:
  name: example-aws-secret
  type: Opaque
  # optional
  labels:
    dockhand: awesome
  annotations:
    alpha: bravo
data:
  alpha: << (aws "dockhand-test" "alpha") >>
  bravo.yaml: |
    bravo: << (aws "dockhand-test?version=latest" "bravo") >>
    charlie: delta
```
Result:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: example-aws-secret
  namespace: aws
  labels:
    dockhand: awesome
  annotations:
    alpha: bravo
data:
  alpha: czNjcjN0Cg==
  bravo.yaml: YnJhdm86IGFub3RoZXItczNjcjN0CmNoYXJsaWU6IGRlbHRhCg==
```

### Azure Key Vault
`DockhandSecret` supports retrieval of Azure Key Vault `json` secret using `<< (azureJson <secret-name> <json-key>) >>` or `text` secret using `<< (azureText <secret-name>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Suppose you have an Azure Key Vault Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }` and an Azure Key Vault Secret named `dockhand-text-test` which has data `"text-s3cr3t"`. The following `DockhandSecret` would generate create an `Opaque` `Secret` in the `azure` namespace.

```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: example-azure-dockhand
  namespace: azure
profile: dockhand-profile
secretSpec:
  name: example-azure-secret
  type: Opaque
data:
  alpha: << (azureJson "dockhand-test" "alpha") >>
  bravo.yaml: |
    bravo: << (azureJson "dockhand-test" "bravo") >>
    charlie: << (azureText "dockhand-text-test") >>
    delta: echo
```

Result:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: example-azure-dockhand
  namespace: azure
data:
  alpha: czNjcjN0Cg==
  bravo.yaml: YnJhdm86IGFub3RoZXItczNjcjN0CmNoYXJsaWU6IHRleHQtczNjcjN0CmRlbHRhOiBlY2hvCg==
```

### GCP Secrets Manager
`DockhandSecret` supports retrieval of GCP Secrets Manager `json` secret using `<< (gcpJson <secret-name> <json-key>) >>` or `text` secret using `<< (gcpText <secret-name>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Suppose you have an GCP Secrets Manager Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }` and an GCP Secrets Manager Secret named `dockhand-text-test` which has data `"text-s3cr3t"`. The following `DockhandSecret` would generate create an `Opaque` `Secret` in the `gcp` namespace.

```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: example-gcp-dockhand
  namespace: gcp
profile: dockhand-profile
secretSpec:
  name: example-gcp-secret
  type: Opaque
data:
  alpha: << (gcpJson "dockhand-test" "alpha") >>
  bravo.yaml: |
    bravo: << (gcpJson "dockhand-test" "bravo") >>
    charlie: << (gcpText "dockhand-text-test") >>
    delta: echo
```

Result:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: example-gcp-dockhand
  namespace: gcp
data:
  alpha: czNjcjN0Cg==
  bravo.yaml: YnJhdm86IGFub3RoZXItczNjcjN0CmNoYXJsaWU6IHRleHQtczNjcjN0CmRlbHRhOiBlY2hvCg==
```

### Vault
`DockhandSecret` supports retrieval of an AWS Secrets Manager `json` secret using `<< (aws <secret-name> <json-key>) >>`. Note that Vault `v2` keystores supports optional `?version=` but `v1` does not.

Suppose you have a Vault Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }`. The following `DockhandSecret` would generate create an `Opaque` `Secret` in the `vault` namespace.
```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: example-vault-dockhand
  namespace: vault
profile: dockhand-profile
secretSpec:
  name: example-aws-secret
  type: Opaque
  # optional
  labels:
    dockhand: awesome
  annotations:
    alpha: bravo
data:
  alpha: << (vault "dockhand-test" "alpha") >>
  bravo.yaml: |
    bravo: << (vault "dockhand-test?version=latest" "bravo") >>
    charlie: delta
```
Result:
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: example-vault-secret
  namespace: vault
  labels:
    dockhand: awesome
  annotations:
    alpha: bravo
data:
  alpha: czNjcjN0Cg==
  bravo.yaml: YnJhdm86IGFub3RoZXItczNjcjN0CmNoYXJsaWU6IGRlbHRhCg==
```

## Helm
{{< hint info >}}
**Info**\
If you are using `DockhandSecrets` in a Helm chart, and you simply want to retrieve the latest version of the secret everytime helm is executed, place an annotation on the `DockhandSecret` that generates a timestamp - this will trigger the operator to handle a `DockhandSecret` change.
{{</hint>}}

```yaml
annotations:
  updateTimestamp: {{ now | date "20060102150405" | quote }}
```

### Helm Chart Example
A helm chart example might look like:
```yaml
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: {{ include "dockhand-demo.fullname" . }}
  labels:
    {{- include "dockhand-demo.labels" . | nindent 4 }}
  annotations:
    updateTimestamp: {{ now | date "20060102150405" | quote }}
profile: dockhand-profile
secretSpec:
  name: {{ include "dockhand-demo.fullname" . }}
  type: Opaque
  labels:
    {{- include "dockhand-demo.labels" . | nindent 4 }}
data:
  alphaDB: 'postgresql://user:<< (aws {{ (printf "dockhand-demo-%s-db-password" .Values.environment) | quote }} "alpha") >>@{{ .Values.postgres.host }}:{{ .Values.postgres.port }}/db'
```