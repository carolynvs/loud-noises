---
title: QuickStart
description: How to make Slack shut up as quickly as possible
menu: main
---

Before you can use Slack Overload, you need to give it access to your Slack
account by clicking the button below, if you haven't already done so:

<p align="center">
  <a href="https://slack.com/oauth/v2/authorize?scope=commands&user_scope=dnd:read,dnd:write,users:write,users.profile:write&client_id=2413351231.504877832356">
    <img alt="Add to Slack" height="40" width="139" src="https://platform.slack-edge.com/img/add_to_slack.png" srcset="https://platform.slack-edge.com/img/add_to_slack.png 1x, https://platform.slack-edge.com/img/add_to_slack@2x.png 2x" />
  </a>
</p>
  
Now that you have the app enabled on one of your teams, open up your Slack
client and follow along!

## Define a status trigger

Triggers let you save a status change so you can quickly change your status
later. 

Let's define a trigger named "lunch" that will set your status to "brb omnomnom"
with the burrito 🌯 emoji for one hour.

```
/create-trigger lunch = brb omnomnom (🌯) for 1h
```

## List your saved triggers

Remind yourself which triggers you have set up:

```
/list-triggers
```

## Trigger the status change

Ok, time for lunch!

```
/trigger lunch
```

## Clear your status

Sadly, you got back from lunch early and want the world to know.

```
/clear-status
```

# Next steps

Hopefully you think this is useful and are ready to install the app on your other
Slack teams. Run the following command to get a magic link that will associate
your teams together so that when you execute a trigger, it is run
on all of your teams.

```
/link-slack
```

Use the drop down at the top right of the page to select which Slack team you
are linking to SlackOverload.

![drop down at top right of slack oauth request page listing slack teams](/img/link-slack.png)