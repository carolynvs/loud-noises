---
title: Privacy Policy
description: I don't want your data
menu: main
---

<p align="center"><strong>
    The app collects just enough information to
    do what you ask it to and doesn't share any of it.
</strong></p>

The app does not collect your name, email address or any any personally
identifiable information because I don't want it or use it. Any data that the
app does store is never shared with anyone else.

The app does make up a uid out of thin air and then associate it with the Slack
user id of each user that you use to log into the app's site with. That is how
the app is able to figure out which Slack account statuses to update when you
run a trigger.

The app does securely store an oauth token that can act on behalf of your user
with the following scopes:

* [dnd:read][dnd-read] - See your Do Not Disturb status.
* [dnd:write][dnd-write] - Set yourself to Do Not Disturb and back.
* [users:write][users-write] - Set yourself to away and back.
* [users.profile:write][profile-write] - Set your status message / emoji.

It only uses the oauth token when you instruct the app to use it on your behalf
with slash commands such as `/trigger` or for a scheduled status change.

That's about it, the less data I have, the better I feel about it. If you have
questions, [open an issue][issue], we'll figure it out and get it documented.

[dnd-read]: https://api.slack.com/scopes/dnd:read
[dnd-write]: https://api.slack.com/scopes/dnd:write
[users-write]: https://api.slack.com/scopes/users:write
[profile-write]: https://api.slack.com/scopes/users.profile:write

[issue]: https://github.com/carolynvs/slackoverload/issues/new
