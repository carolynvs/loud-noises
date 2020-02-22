#!/bin/bash

exec timeout 300 bash -c 'while [[ "$(curl -s -o /dev/null -w ''%{http_code}'' https://cmd.slackoverload.com/health)" != "200" ]]; do sleep 5; done' || false
