# Nomad Autoscaler [![Build Status](https://circleci.com/gh/hashicorp/nomad-autoscaler.svg?style=svg)](https://circleci.com/gh/hashicorp/nomad-autoscaler) [![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)

The Nomad Autoscaler is an autoscaling daemon for [Nomad](https://nomadproject.io/),
architectured around plug-ins to allow for easy extensibility in terms of supported metrics sources,
scaling targets and scaling algorithms.

***This project is in the early stages of development and is supplied without guarantees and subject to change without warning***

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

## Requirements

The autoscaler relies on Nomad APIs that were introduced in Nomad 0.11-beta1, some of which have been changed during the beta.
The compatibility requirements are as follows:

|                                     Autoscaler Version                                    | Nomad Version |
|:-----------------------------------------------------------------------------------------:|:-------------:|
| [0.0.1-techpreview1](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview1/) | 0.11.0-beta1  |
| [0.0.1-techpreview2](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview2/) |    0.11.0     |
| [0.0.2](https://releases.hashicorp.com/nomad-autoscaler/0.0.2/)                           |    0.11.2     |
| [0.1.0](https://releases.hashicorp.com/nomad-autoscaler/0.1.0/)                           |    0.12.0     |
| [nightly](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly)             |    0.12.0     |

## Documentation

Documentation is available on the [Nomad project website](https://www.nomadproject.io/docs/autoscaling).

## Demo

The [Vagrant based demo](./demo/vagrant/README.md) provides a guided example of running and autoscaling
an application based on Prometheus metrics using the Nomad Autoscaler.

The [remote provider based demo](./demo/remote/README.md) provides guided examples of running horizontal
application and cluster scaling.

## Building

The Nomad Autoscaler can be easily built for local testing or development using the `make build`
command. This will output the compiled binary to `./bin/nomad-autoscaler`.

## Nightly Builds

As a tech preview, this project is under constant updates, so every day the
[`nightly` release](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly) is updated
with binaries built off the latest code in the `master` branch. This should make it easier for you
to try new features and bug fixes.
