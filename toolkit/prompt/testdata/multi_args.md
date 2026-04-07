---
name: multi_args
title: Multi Arg Agent
description: Agent with several typed inputs
input:
  type: object
  properties:
    zebra:
      type: string
      title: Zebra title
    apple:
      type: string
      description: Apple description
    mango:
      type: string
      description: Mango description
      title: Mango title
  required:
    - apple
---
Template body.
