---
title: Slack Overload 101
description: How to make Slack shut up as quickly as possible
---

Now that you have Slack Overload enabled on one of your teams, what next? Open
up your Slack client and follow along...

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