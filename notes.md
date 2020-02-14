# Auth Dance
https://slack.com/oauth/authorize?client_id=2413351231.504877832356&user_scope=dnd:write,users:write,users.profile:write

## bot scopes
* commands - Enable slash commands

## user scopes
* dnd:read - See your DND status
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

/create-global-trigger busy = (😱)
```

## Clear your status

These are built-in so that you don't need to make a trigger

```
/clear-status
/clear-global-status
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
/list-global-triggers
```

# Infra

## Domain

* slackoverload.com -> Netlify
* cmd.slackoverload.com -> ACI

## ACI

* Runs the app in a container
* Runs with a [managed identity](https://docs.microsoft.com/en-us/azure/container-instances/container-instances-managed-identity)
  so that the process transparently has access to keyvault
* Deploy with ./redeploy.sh

## Data

* OAuth tokens -> keyvault
* User configuration -> blob storage
    * triggers: userid/trigger
    * schedules: userid/schedule
    * users: userid/user

## User Management

### Create User

1. Add to slack
1. Generate user id
1. Store user object in blob storage
    * collection of all the slack user ids associated so far
1. Store oauth token in keyvault as oauth-slackid
    * tag the secret with the user id

### Lookup user

1. Get slack user id from incoming slash webhook
1. 