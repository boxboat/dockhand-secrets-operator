# dockhand-secrets-operator-crd
Installs the CRDs required for the [dockhand-secrets-operator](https://github.com/boxboat/dockhand-secrets-operator)

Read the [docs](https://secrets-operator.dockhand.dev)

## Install Instructions
```
helm repo add dockhand https://boxboat.github.io/dockhand-charts
helm repo update
helm install --namespace dockhand-secrets-operator dockhand/dockhand-secrets-operator-crd
```
