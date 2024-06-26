---
title: Core Concepts
weight: -90
---
<!--more-->

{{< toc >}}

## Single vs Multi-Tenant
With the `v1alpha2` types, the `dockhand-secrets-operator` supports the multi-tenant use case by requiring a `Profile` to be defined in each namespace that utilizes a Dockhand `Secret` by default. If you do not require multi-tenant security then you can enable cross-namespace access through the [helm chart](https://github.com/boxboat/dockhand-charts/blob/master/dockhand-secrets-operator/values.yaml) or by passing `--allow-cross-namespace` to the controller. This will allow Dockhand `Secrets` to reference a `Profile` in any namespace where the operator has read access.

## Dockhand Profile
A `Profile` can contain one or more secrets backends and provides the `dockhand-secrets-operator` with the information it needs to connect to a Secrets Manager.

For simplicity the examples below assume a single tenant use case where the `Profile` exists in the `dockhand-secrets-operator` namespace. 

### Example: Dockhand Profile
```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Profile
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

## Secret

Dockhand `Secret` is essentially a Go template with alternate delimiters `<< >>` so that you can use it in a Helm chart. The operator is built off [dockcmd](https://github.com/boxboat/dockcmd). Sprig functions are supported and specific versions of secrets are supported through the use of `?version=` on the secret name. For simplicity `?version=latest` will work with all of the backends but specific versions require the value expected by the backend.

Note that GCP and Azure have 2 forms of supported secrets `text` or `json`. The text version will return the entire secret stored in the key/value whereas the json version will interpret the stored value as `json` and allow you retrieve a single `key` in the secret.

The Dockhand `Secret` will generate a secret of type `secretSpec.type` in the same namespace specified by `secretSpec.name`. Changes to a Dockhand `Secret` will trigger a refresh of the `Secret` managed by that Dockhand `Secret`. You can optionally have labels or annotations injected on the `Secret` created by the Dockhand `Secret`.

The `profile` field allows you to specify different `Profiles`, which gives you flexibility to connect to numerous Secrets Managers from the same cluster.

The `syncInterval` field instructs the operator to poll for changes to that particular Dockhand `Secret` - something greater than `5s`. The default is `0s`, which means do not poll. 

#### `syncInterval` Considerations: 
* Cloud Providers charge for secrets retrieval requests
* `cacheTTL` is specified in the `Profile` so be aware of your TTL when picking a `syncInterval`.
* See [Auto Updates](#auto-updates) section below

### AWS Secrets Manager
Dockhand `Secret` supports retrieval of an AWS Secrets Manager `json` secret using `<< (aws <secret-name> <json-key>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Note `<secret-name>` can also be the ARN of the secret.

Suppose you have an AWS Secrets Manager Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }`. The following Dockhand `Secret` would generate create an `Opaque` `Secret` in the `aws` namespace.
```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: example-aws-dockhand
  namespace: aws
profile: 
  name: dockhand-profile
  namespace: dockhand-secrets-operator
# default 0s - set to an interval greater than 5s to enable polling for changes
syncInterval: 0s
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
Dockhand `Secret` supports retrieval of Azure Key Vault `json` secret using `<< (azureJson <secret-name> <json-key>) >>` or `text` secret using `<< (azureText <secret-name>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Suppose you have an Azure Key Vault Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }` and an Azure Key Vault Secret named `dockhand-text-test` which has data `"text-s3cr3t"`. The following Dockhand `Secret` would generate create an `Opaque` `Secret` in the `azure` namespace.

```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: example-azure-dockhand
  namespace: azure
profile: 
  name: dockhand-profile
  namespace: dockhand-secrets-operator
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
Dockhand `Secret` supports retrieval of GCP Secrets Manager `json` secret using `<< (gcpJson <secret-name> <json-key>) >>` or `text` secret using `<< (gcpText <secret-name>) >>`. The `<secret-name>` supports optional `?version=<version-id>` query string.

Suppose you have an GCP Secrets Manager Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }` and an GCP Secrets Manager Secret named `dockhand-text-test` which has data `"text-s3cr3t"`. The following Dockhand `Secret` would generate create an `Opaque` `Secret` in the `gcp` namespace.

```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: example-gcp-dockhand
  namespace: gcp
profile:
  name: dockhand-profile
  namespace: dockhand-secrets-operator
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
Dockhand `Secret` supports retrieval of an AWS Secrets Manager `json` secret using `<< (aws <secret-name> <json-key>) >>`. Note that Vault `v2` keystores supports optional `?version=` but `v1` does not.

Suppose you have a Vault Secret named `dockhand-test`, which has `json` data `{ "alpha": "s3cr3t", "bravo": "another-s3cr3t" }`. The following Dockhand `Secret` would generate create an `Opaque` `Secret` in the `vault` namespace.
```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: example-vault-dockhand
  namespace: vault
profile:
  name: dockhand-profile
  namespace: dockhand-secrets-operator
# default 0s - set to an interval greater than 5s to enable polling for changes
syncInterval: 0s
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
If you are using Dockhand `Secrets` in a Helm chart, and you simply want to retrieve the latest version of the secret everytime helm is executed, place an annotation on the Dockhand `Secret` that generates a timestamp - this will trigger the operator to handle a Dockhand `Secret` change.
{{</hint>}}

```yaml
annotations:
  updateTimestamp: {{ now | date "20060102150405" | quote }}
```

### Helm Chart Example
A helm chart example might look like:
```yaml
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: {{ include "dockhand-demo.fullname" . }}
  labels:
    {{- include "dockhand-demo.labels" . | nindent 4 }}
  annotations:
    # use an annotation like this to force a sync everytime a helm deployment is made
    updateTimestamp: {{ now | date "20060102150405" | quote }}
profile: 
  name: dockhand-profile
  namespace: dockhand-secrets-operator
# default 0s - set to an interval greater than 5s to enable polling for changes
syncInterval: 0s
secretSpec:
  name: {{ include "dockhand-demo.fullname" . }}
  type: Opaque
  labels:
    {{- include "dockhand-demo.labels" . | nindent 4 }}
data:
  alphaDB: 'postgresql://user:<< (aws {{ (printf "dockhand-demo-%s-db-password" .Values.environment) | quote }} "alpha") >>@{{ .Values.postgres.host }}:{{ .Values.postgres.port }}/db'
```


## Auto Updates
For `DaemonSets`, `Deployments` and `StatefulSets` you can insert the following label, which make the `dockhand-secrets-operator` auto roll those types when a Dockhand `Secret` updates the `Secret` it owns. If this option is combined with a `syncInterval` greater than `5s`, then the operator will roll these types over automatically when it updates the k8s `Secret` with changes from your Secrets Backend.

```yaml
metadata:
  labels:
    dhs.dockhand.dev/autoUpdate: "true"
```