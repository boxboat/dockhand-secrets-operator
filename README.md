# dockhand-secrets-operator
![Main](https://github.com/boxboat/dockhand-secrets-operator/actions/workflows/docker.yaml/badge.svg?branch=master)
![Helm](https://github.com/boxboat/dockhand-secrets-operator/actions/workflows/helm.yaml/badge.svg?branch=master)

Secrets management with GitOps can be challenging in Kubernetes environments. Often engineers resort to manual secret creation,  injection of secrets through scripts with the CI/CD tool or even worse just committing the secrets directly to git.

The Dockhand Secrets Operator solves that problem by allowing you to make arbitrary secrets in a familiar way with only the secret bits stored in the backend(s) of your choice - AWS Secrets Manager, Azure Key Vault, GCP Secrets Manager or Vault. Secret references can be stored in git with your Helm chart or Kubernetes manifests. 

The operator supports auto rolling updates for `Deployments`, `StatefulSets` and `DaemonSets` through the use of a single `label` added to the metadata of those items. The operator accomplishes this by injecting an annotation with the checksum of the `Secrets` referenced in those manifests and will update that checksum annotation automatically when the secret changes.

The operator installation deploys the CRDs, the controller and a mutating webhook to provide auto updates for types mentioned above.

## Usage
1. Install Helm charts from [dockhand-charts](https://github.com/boxboat/dockhand-secrets-operator/charts)
2. Configure Operator with `DockhandSecretsProfile` to connect `dockhand-secrets-operator` to 1 or more Secrets Managers
3. Create `DockhandSecrets` to manage `Secrets` required by your applications.


### CustomResourceDefinitions
`dockhand-secrets-operator` makes use of 2 CRDs to manage `Secrets`. One provides the data required for the operator to connect to the secrets manager(s) and the other manages `Secrets`

#### Dockhand Profile

Example of how to create a `Profile`. 

Note that `dockhand-secrets-operator` has to 2 main operating modes. One that will allow cross namespace access to Dockhand `Profiles` and the default mode which blocks cross namespace access. The default mode supports  multi-tenant usage by requiring a `Profile` in each namespace. If you are running a single-tenant cluster then the flag `--allow-cross-namespace` will allow you to specify a `Profile` in another namespace for the operator to utilize.

The `cacheTTL` field allows you to control how long a Secrets Manager response is cached by the operator. The default is 60 seconds which prevents the `dockhand-secrets-operator` from abusing the Secrets Backend.

```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Profile
metadata:
  name: test-dockhand-profile
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

#### Dockhand Secret
The Dockhand `Secret` is essentially a go template with alternate delimiters `<< >>` so that you can use it in a Helm chart. The operator is built off [dockcmd](https://github.com/boxboat/dockcmd). Sprig functions are supported and specific versions of secrets are supported through the use of `?version=` on the secret name. For simplicity `?version=latest` will work with all the backends but specific versions require the value expected by the backend.

Note that GCP and Azure have 2 forms of supported secrets `text` or `json`. The text version will return the entire secret stored in the key/value where as the json version will interpret the stored value as `json` and allow you retrieve a single `key` in the secret.

The Dockhand `Secret` will generate a secret of type `secretSpec.type` in the same namespace specified by `secretSpec.name`. Changes to a Dockhand `Secret` will trigger a refresh of the `Secret` managed by that Dockhand `Secret`. You can optionally have labels or annotations injected on the `Secret` created by the Dockhand `Secret`.

The `profile` field allows you to specify the Dockhand `SecretProfile`, which gives you flexibility to connect to numerous Secrets Managers from the same cluster.

```yaml
---
apiVersion: dhs.dockhand.dev/v1alpha2
kind: Secret
metadata:
  name: dockhand-example-secret
profile: 
  name: dockhand-profile
  namespace: dockhand-secrets-operator
secretSpec:
  name: example-secret
  type: Opaque
  # optional
  labels:
    foo: bar
  # optional
  annotations:
    alpha: charlie
data:
  # aws secrets can also be accessed by arn
  aws-alpha: << (aws "dockcmd-test" "alpha") >>
  aws-bravo: << (aws "dockcmd-test" "bravo") >>
  aws-charile: << (aws "dockcmd-test" "charlie") >>
  azure-alpha-text: << (azureText "dockcmd-text-test") | squote >>
  azure-alpha: << (azureJson "dockcmd-json-test" "alpha" ) | squote >>
  azure-bravo: << (azureJson "dockcmd-json-test" "bravo" ) | squote >>
  azure-charlie: << (azureJson "dockcmd-json-test" "charlie" ) | squote >>
  gcp-alpha-text: << (gcpText "dockcmd-text") | squote >>
  gcp-alpha: << (gcpJson "dockcmd-json?version=latest" "alpha" ) | squote >>
  gcp-bravo: << (gcpJson "dockcmd-json" "bravo" ) | squote >>
  gcp-charlie: << (gcpJson "dockcmd-json" "charlie" ) | squote >>
  vault-alpha: << (vault "secret/dockcmd-test?version=1" "alpha" ) | squote >>
  vault-bravo: << (vault "secret/dockcmd-test?version=1" "bravo" ) | squote >>
  vault-charlie: << (vault "secret/dockcmd-test?version=1" "charlie" ) | squote >>
```

If you are using Dockhand `Secrets` in a Helm chart, and you simply want to retrieve the latest version of the secret everytime helm is executed, place an annotation on the Dockhand `Secret` that generates a timestamp - this will trigger the operator to handle a Dockhand `Secret` change.
```yaml
annotations:
  updateTimestamp: {{ now | date "20060102150405" | quote }}
```

### Auto Updates
For `DaemonSets`, `Deployments` and `StatefulSets` you can insert the following label, which make the `dockhand-secrets-operator` auto roll those types when a Dockhand `Secret` updates the `Secret` it owns.

```yaml
metadata:
  labels:
    dhs.dockhand.dev/autoUpdate: "true"
```