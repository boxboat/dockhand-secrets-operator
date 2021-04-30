# dockhand-secrets-operator
Secrets management with GitOps can be challenging in Kubernetes environments. Often engineers resort to manual secret creation,  injection of secrets through scripts with the CI/CD tool or even worse just committing the secrets directly to git.

The Dockhand Secrets Operator solves that problem by allowing you to make arbitrary secrets in a familiar way with only the secret bits stored in the backend(s) of your choice - AWS Secrets Manager, Azure Key Vault, GCP Secrets Manager or Vault. Secret references can be stored in git with your Helm chart or Kubernetes manifests. 

The operator supports auto rolling updates for `Deployments`, `StatefulSets` and `DaemonSets` through the use of a single `label` added to the metadata of those items. The operator accomplishes this by injecting an annotation with the checksum of the `Secrets` referenced in those manifests and will update that checksum annotation automatically when the secret changes. 

## Usage
1. Install Helm Chart
2. Configure Operator with `DockhandProfile` to connect `dockhand-secrets-operator` to 1 or more Secrets Managers
3. Create `DockhandSecrets` to manage `Secrets` required by your applications.


### CustomResourceDefinitions
`dockhand-secrets-operator` makes use of 2 CRDs to manage `Secrets`. One provides the data required for the operator to connect to the secrets manager(s) and the other manages `Secrets`

#### DockhandProfile
Example of how to create a `DockhandProfile`
```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandProfile
metadata:
  name: test-dockhand-profile
  namespace: dockhand-secrets-operator
awsSecretsManager:
  cacheTTL: 60
  region: us-east-1
  accessKeyId: <accessKeyId>
  secretAccessKeyRef: dockhand-profile-secrets
azureKeyVault:
  cacheTTL: 60
  keyVault: dockcmd
  tenant: <tenantId>
  clientId: <clientId>
  clientSecretRef: dockhand-profile-secrets
gcpSecretsManager:
  cacheTTL: 60
  project: myproject
  credentialsFileSecretRef: dockhand-profile-secrets
vault:
  cacheTTL: 60
  addr: http://vault:8200
  tokenRef: dockhand-profile-secrets
---
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: dockhand-profile-secrets
  namespace: dockhand-secrets-operator
data:
  AWS_SECRET_ACCESS_KEY: <Base64 AWS SECRET KEY>
  VAULT_TOKEN: <Base64 encoded vault token>
  AZURE_CLIENT_SECRET: <Base64 encoded azure client secret>
  gcp-credentials.json: <Base64 encoded GCP JSON file>
```

#### DockhandSecret
`DockhandSecret` is essentially a go template with alternate delimiters `<< >>` so that you can use it in a Helm chart. The operator is built off [dockcmd](https://github.com/boxboat/dockcmd). Sprig functions are supported and specific versions of secrets are supported through the use of `?version=` on the secret name. For simplicity `?version=latest` will work with all of the backends but specific versions require the value expected by the backend.

Note that GCP and Azure have 2 forms of supported secrets `text` or `json`. The text version will return the entire secret stored in the key/value where as the json version will interpret the stored value as `json` and allow you retrieve a single `key` in the secret.

```yaml
---
apiVersion: dockhand.boxboat.io/v1alpha1
kind: DockhandSecret
metadata:
  name: test-dockhand-secret-aws
dockhandProfile: test-dockhand-profile
secretName: test-secret-aws
data:
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

If you are using `DockhandSecrets` in a Helm chart and you simply want to retrieve the latest version of the secret everytime helm is executed, place an annotation on the `DockhandSecret` that generates a timestamp - this will trigger the operator to handle a `DockhandSecret` change.
```yaml
annotations:
  updateTimestamp: {{ now | date "20060102150405" }}
```