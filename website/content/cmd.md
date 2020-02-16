---
title: Commands
description: Documentation for all the commands available in the SlackOverload app
menu: main
---

* [Create Trigger](#create-trigger)
* [Delete Trigger](#delete-trigger)
* [List Triggers](#list-triggers)
* [Trigger](#trigger)

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