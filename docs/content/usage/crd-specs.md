---
title: CustomResourceDefinitions
weight: -50
---
<!--more-->

{{< toc >}}

## Profile

```
KIND:     Profile
VERSION:  dhs.dockhand.dev/v1alpha2

DESCRIPTION:
     Holds configuration details for a Profile

FIELDS:
   apiVersion	<string>
     APIVersion defines the versioned schema of this representation of an
     object. Servers should convert recognized schemas to the latest internal
     value, and may reject unrecognized values. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources

   awsSecretsManager	<Object>
     AWS Secrets Manager configuration to allow the Dockhand Secrets Operator to
     retrieve Secrets from AWS

   azureKeyVault	<Object>
     Azure Key Vault configuration to allow the Dockhand Secrets Operator to
     retrieve Secrets from Azure

   gcpSecretsManager	<Object>
     Google Cloud Platform Secrets Manager Configuration to allow Dockhand
     Secrets Operator to retrieve secrets from GCP. Authentication can be
     Application Default Credentials or by providing a key.json

   kind	<string>
     Kind is a string value representing the REST resource this object
     represents. Servers may infer this from the endpoint the client submits
     requests to. Cannot be updated. In CamelCase. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds

   metadata	<Object>
     Standard object's metadata. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata

   vault	<Object>
     HashiCorp Vault Configuration to allow Dockhand Secrets Operator to
     retrieve secrets from Vault. Secrets can be retrieved with either a
     roleId/secretId or with a Vault Token.
```

### Profile.awsSecretsManager
```
KIND:     Profile
VERSION:  dhs.dockhand.dev/v1alpha2

RESOURCE: awsSecretsManager <Object>

DESCRIPTION:
     AWS Secrets Manager configuration to allow the Dockhand Secrets Operator to
     retrieve Secrets from AWS. If no accessKeyId and secretAccessKey are provided
     then chain credentials will be used.

FIELDS:
   accessKeyId	<string>
     AWS IAM Access Key

   cacheTTL	<string>
     Duration to cache secret responses

   region	<string>
     AWS Region to retrieve secrets from

   secretAccessKeyRef	<Object>
     Name of secret containing AWS IAM Secret Access Key
```

### Profile.azureKeyVault
```
KIND:     Profile
VERSION:  dhs.dockhand.dev/v1alpha2

RESOURCE: azureKeyVault <Object>

DESCRIPTION:
     Azure Key Vault configuration to allow the Dockhand Secrets Operator to
     retrieve Secrets from Azure

FIELDS:
   cacheTTL	<string>
     Duration to cache secret responses

   clientId	<string>
     Azure Client ID to access the Key Vault

   clientSecretRef	<Object>
     Reference to Azure Client Secret

   keyVault	<string>
     Name of Azure Key Vault to retrieve secrets from

   tenant	<string>
     Azure Tenant ID where the Key Vault resides
```

### Profile.gcpSecretsManager
```
KIND:     Profile
VERSION:  dhs.dockhand.dev/v1alpha2

RESOURCE: gcpSecretsManager <Object>

DESCRIPTION:
     Google Cloud Platform Secrets Manager Configuration to allow Dockhand
     Secrets Operator to retrieve secrets from GCP. Authentication can be
     Application Default Credentials or by providing a key.json

FIELDS:
   cacheTTL	<string>
     Duration to cache secret responses

   credentialsFileSecretRef	<Object>
     Secret Reference containing JSON credentials file stored in a key named
     gcp-credentials.json

   project	<string>
     The GCP Project to reference for this profile
```

### Profile.vault
```
KIND:     Profile
VERSION:  dhs.dockhand.dev/v1alpha2

RESOURCE: vault <Object>

DESCRIPTION:
     HashiCorp Vault Configuration to allow Dockhand Secrets Operator to
     retrieve secrets from Vault. Secrets can be retrieved with either a
     roleId/secretId or with a Vault Token.

FIELDS:
   addr	<string>
     Vault Address e.g. http://vault:8200

   cacheTTL	<string>
     Duration to cache secret responses

   roleId	<string>
     Vault Role ID

   secretIdRef	<Object>
     Reference to secret containing the Vault secretId

   tokenRef	<Object>
     Reference to secret containing the Vault Token
```

## Dockhand Secret
```
KIND:     Secret
VERSION:  dhs.dockhand.dev/v1alpha2

DESCRIPTION:
     Secret Object

FIELDS:
   apiVersion	<string>
     APIVersion defines the versioned schema of this representation of an
     object. Servers should convert recognized schemas to the latest internal
     value, and may reject unrecognized values. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources

   data	<map[string]string>
     Store arbitrary templated secret data here just as you would in a
     kubernetes configmap. The dockhand-secrets-operator will retrieve the
     secrets from the secrets backend and create normal kubernetes secrets for
     use by your application. Secrets should be templated using go templating
     with alternative delimiters << >> rather than \{\{ \}\}.

   kind	<string>
     Kind is a string value representing the REST resource this object
     represents. Servers may infer this from the endpoint the client submits
     requests to. Cannot be updated. In CamelCase. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds

   metadata	<Object>
     Standard object's metadata. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata

   profile	<Object>
     Profile to use for this secret

   secretSpec	<Object>
     Specification to use for creating the Kubernetes Secret

   status	<Object>
     Provides basic status for a DockhandSecret

   syncInterval	<string>
     Specifies the time interval for polling the secrets backend for changes.
     The default value of 0 indicates that no polling will occur and is the
     default behavior prior to 1.1.0 release, in this case the operator will
     only query the backend when a field in the Dockhand Secret CRD has been
     modified. Valid time units are ns, us, ms, s, m, h, but must exceed 5s
     (when not 0). Also note that the operator will not poll the backend more
     frequently than the cacheTTL of the profile referenced by the Secret
```

### Secret.secretSpec
```
KIND:     Secret
VERSION:  dhs.dockhand.dev/v1alpha2

RESOURCE: secretSpec <Object>

DESCRIPTION:
     Specification to use for creating the Kubernetes Secret

FIELDS:
   annotations	<>
     Optional additional annotations to add to the secret managed by this
     Secret

   labels	<>
     Optional additional labels to add to the secret managed by this
     Secret

   name	<string>
     Name of the secret that will be created or updated with the processed
     contents of the data field.

   type	<string>
     Type of k8s secret to create Opaque, kubernetes.io/service-account-token,
     kubernetes.io/dockercfg, kubernetes.io/dockerconfigjson,
     kubernetes.io/basic-auth, kubernetes.io/ssh-auth, kubernetes.io/tls or
     bootstrap.kubernetes.io/token
```