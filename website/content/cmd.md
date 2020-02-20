---
title: Commands
description: Documentation for all the commands available in the SlackOverload app
menu: main
---

These are all of the slash commands available inside of Slack that you can run
to interact with the Slack Overload app. If you just getting started, use the
[QuickStart](/quickstart/) to learn how to use the Slack Overload app.

* [Clear Status](#clear-status)
* [Create Trigger](#create-trigger)
* [Delete Trigger](#delete-trigger)
* [Link Slack](#link-slack)
* [List Triggers](#list-triggers)
* [Trigger](#trigger)

## Clear Status

Clear your status text, emoji and remove Do Not Disturb.

```
/clear-status
```

## Create Trigger

Define a saved status that you can trigger later using just its name.

```
/create-trigger NAME = STATUS TEXT (:EMOJI:) [DND] [for DURATION]
```

* **NAME**: The name of the trigger. Required.
* **STATUS TEXT**: The status text to set on your profile. Optional.
* **EMOJI**: A single emoji, either a slack encoded emoji like `:boat:` or the
  unicode emoji ⛵️. Required because emoji are great.
* **DND**: Specifies if you should be set to Do Not Disturb. Optional.
* **DURATION**: Default time that the trigger should apply. Optional. Supported
  units are m=minute, h=hour, d=day, w=week, for example 5m would be a duration
  of 5 minutes.

**Examples**
```
/create-trigger lunch = brb omnomnom (🌯) for 1h
/create-trigger vacation = I'm on a boat! (:boat:) DND for 1w
/create-trigger sick = I'm sick, go talk to my manager (🤒) DND
/create-trigger brb = (🚽)
```

## Delete Trigger

Delete a trigger by name.

```
/delete-trigger NAME
```

* **Name**: The name of the trigger. Required.

## Link Slack

Displays a magic link to associate another Slack account to the current one so
that when you run Slack Overload commands, like `/trigger`, it is applied to
that Slack account as well.

```
/link-slack
```

## List Triggers

List all defined triggers.

```
/list-triggers
```

## Trigger

Trigger a predefined status change by name.

```
/trigger NAME
```

* **Name**: The name of the trigger. Required.