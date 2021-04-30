# dockhand-secrets-operator
Secrets management with GitOps can be challenging in Kubernetes environments. Often engineers resort to manual secret creation or injection of secrets through scripts with the CI/CD tool. 

The Dockhand Secrets Operator solves that problem by allowing you to make arbitrary secrets in a familiar way with only the secret bits stored in the backend of your choice - AWS Secrets Manager, Azure Key Vault, GCP Secrets Manager or Vault. Secret references can be stored in git with your Helm chart or Kubernetes manifests. The Dockhand Secrets Operator will manage the Kubernetes Secret for you! 
