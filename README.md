# Nomad Autoscaler [![Build Status](https://circleci.com/gh/hashicorp/nomad-autoscaler.svg?style=svg)](https://circleci.com/gh/hashicorp/nomad-autoscaler) [![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)

The Nomad Autoscaler is an autoscaling daemon for [Nomad](https://nomadproject.io/),
architectured around plug-ins to allow for easy extensibility in terms of supported metrics sources,
scaling targets and scaling algorithms.

The Nomad Autoscaler currently supports:
* **Horizontal Application Autoscaling**: The process of automatically controlling the number of
instances of an application to have sufficient work throughput to meet service-level agreements (SLA).
In Nomad, horizontal application autoscaling can be achieved by modifying the number of allocations
in a task group based on the value of a relevant metric, such as CPU and memory utilization or number
of open connections.

* **Horizontal Cluster Autoscaling**: The process of adding or removing Nomad clients from a cluster
to ensure there is an appropriate amount of cluster resource for the scheduled applications. This is
achieved by interacting with remote providers to start or terminate new Nomad clients based on metrics
such as the remaining free schedulable CPU or memory.

* **Dynamic Application Sizing (Enterprise)**: Dynamic Application Sizing enables organizations to
optimize the resource consumption of applications using sizing recommendations from Nomad. It
evaluates, processes and stores historical task resource usage data, making recommendations for CPU
and Memory resource parameters. The recommendations can be calculated using a number of different
algorithms to ensure the recommendation best fits the application profile.

## Requirements

The autoscaler relies on Nomad APIs that were introduced in Nomad 0.11-beta1, some of which have been changed during the beta.
The compatibility requirements are as follows:

|                                     Autoscaler Version                                    | Nomad Version |
|:-----------------------------------------------------------------------------------------:|:-------------:|
| [0.0.1-techpreview1](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview1/) | 0.11.0-beta1  |
| [0.0.1-techpreview2](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview2/) |    0.11.0     |
| [0.0.2](https://releases.hashicorp.com/nomad-autoscaler/0.0.2/)                           |    0.11.2     |
| [0.1.0](https://releases.hashicorp.com/nomad-autoscaler/0.1.0/)                           |    0.12.0     |
| [0.1.1](https://releases.hashicorp.com/nomad-autoscaler/0.1.1/)                           |    0.12.0     |
| [0.2.0+](https://releases.hashicorp.com/nomad-autoscaler/0.2.0/)                          |    1.0.0+     |
| [nightly](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly)             |    1.0.0+     |

## Documentation

Documentation is available on the [Nomad project website](https://www.nomadproject.io/docs/autoscaling).

## Demo

There are both [horizontal application scaling](https://github.com/hashicorp/nomad-autoscaler-demos/tree/main/vagrant/horizontal-app-scaling) and
[dynamic application sizing](https://github.com/hashicorp/nomad-autoscaler-demos/tree/main/vagrant/dynamic-app-sizing) based demos available
providing guided examples of running the autoscaler.

The [cloud provider based demo](https://github.com/hashicorp/nomad-autoscaler-demos/tree/main/cloud) provides guided examples of running horizontal
application and cluster scaling.

## Building

The Nomad Autoscaler can be easily built for local testing or development using the `make dev`
command. This will output the compiled binary to `./bin/nomad-autoscaler`.

## Nightly Builds and Docker Image Preview

The Nomad Autoscaler is under constant updates, so every day the [`nightly`
release](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly) is updated with
binaries built off the latest code in the `main` branch. This should make it easier for you to try
new features and bug fixes.

Each commit to `main` also generates a preview Docker image that can be accessed from the
[`hashicorppreview/nomad-autoscaler`](https://hub.docker.com/r/hashicorppreview/nomad-autoscaler/tags)
repository on Docker Hub.
