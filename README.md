# service-account-token-operator
This is the repo containing the source code of the operator(controller) that creates long-lived tokens for service-accounts.

## Description
This operator works by watching all the service accounts in the cluster. If a service account is created with an annotation / the following annotation is added to it: `or.io/create-secret: ""`, then the controller will attempt to create a long-lived token for that service account.
It will do it by creating a secret of type `serviceAccountToken` for it. 
The operator will only attempt to create the secret, meaning if a secret with the name `<service-account-name>-token` already exists in the namespace, it will skip creating it. 

## How to install
TBA

## Permissions needed
The service account for the controller needs minimal permissions: Get,List,Watch on `serviceAccounts` and Create on `secrets`.

## Reconciliation flow
The operator will only reconcile serviceAccounts that have the `or.io/create-secret: ""` annotation, it will do it by using a predicate function that will filter serviceAccounts and pass through only serviceAccounts with the annotation. 
Then the operator will fetch the serviceAccount that called for the reconciliation.
Lastly, the operator will attempt to create a secret (of type serviceAccountToken) for the cooresponding serviceAccount, if the creation fails due to the secret already existing, the operator will exit cleanly with a message, otherwise if failed because of another reason, the error will be outputted to help debugging. 
