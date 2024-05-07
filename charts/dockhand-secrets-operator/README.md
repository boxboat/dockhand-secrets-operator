# dockhand-secrets-operator
Installs the [dockhand-secrets-operator](https://github.com/boxboat/dockhand-secrets-operator)

Read the [docs](https://secrets-operator.dockhand.dev)


## Install Instructions
```
helm repo add dso https://boxboat.github.io/dockhand-secrets-operator/charts
helm repo update
helm install --namespace dockhand-secrets-operator dso/dockhand-secrets-operator-crd
helm install --namespace dockhand-secrets-operator dso/dockhand-secrets-operator
```
