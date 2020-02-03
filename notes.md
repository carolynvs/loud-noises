# Auth Dance
https://slack.com/oauth/authorize?client_id=2413351231.504877832356&user_scope=dnd:write,users:write,users.profile:write

## scopes
* dnd:write - Set yourself to DND and back
* users:write - Set yourself to away and back
* users.profile:write - Set your status message / emoji

# Managing Da Noise

## Tell certain workspaces to listen for a named trigger

```
/create-trigger vacation = vacay (🌴) DND

/create-global-trigger lunch = brb eating lunch (🌯) for 1h

/create-global-trigger sick = out sick (🤒) DND

/create-trigger off-work = DND
```

## Manual trigger

```
/trigger vacation for 1w
/trigger lunch
/trigger sick for 3d
```

## Schedule triggers

```
/schedule-trigger off-work MTuWTh 5pm until tomorrow
/schedule-trigger off-work F 5pm for 2d
```

## Show me my triggers

```
/list-triggers
```

# Infra

## Frontdoor

https://slackoverload.com

* /* -> storage container
* /slack/* -> ACI

## ACI

* Runs the app in a container
* Runs with a [managed identity](https://docs.microsoft.com/en-us/azure/container-instances/container-instances-managed-identity)
  so that the process transparently has access to keyvault
  
# Deployments

docker build -t carolynvs/slackoverload:v0.0.2 .
docker push carolynvs/slackoverload:v0.0.2
az container dleete -g slackoverload --nme slackoverload
az container create -g slackoverload --name slackoverload --image carolynvs/slackoverload:v0.0.2 --dns-name-label slackoverload --ports 8080 --assign-identity /subscriptions/83f90879-de5f-4c9e-9459-593fb2a17c89/resourcegroups/slackoverload/providers/Microsoft.ManagedIdentity/userAssignedIdentities/slackoverload-api
