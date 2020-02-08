#!/bin/bash

set -euo pipefail

VERSION=v0.0.2

docker build -t carolynvs/slackoverload:$VERSION .
docker push carolynvs/slackoverload:$VERSION
az container delete -g slackoverload --name slackoverload -y
az container create -g slackoverload --name slackoverload \
  --image carolynvs/slackoverload:$VERSION --dns-name-label slackoverload --ports 8080 \
  --assign-identity /subscriptions/83f90879-de5f-4c9e-9459-593fb2a17c89/resourcegroups/slackoverload/providers/Microsoft.ManagedIdentity/userAssignedIdentities/slackoverload-api
