# Nomad Autoscaler [![Build Status](https://circleci.com/gh/hashicorp/nomad-autoscaler.svg?style=svg)](https://circleci.com/gh/hashicorp/nomad-autoscaler) [![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)

The Nomad Autoscaler is an autoscaling daemon for [Nomad](https://nomadproject.io/), architectured around plug-ins to allow for easy extensibility in terms of supported metrics sources, scaling targets and scaling algorithms.

***This project is in the early stages of development and is supplied without guarantees and subject to change without warning***

Known issues and limitations:
 * [scaling cooldowns](https://github.com/hashicorp/nomad-autoscaler/issues/12) are not implemented, this makes the autoscaling of applications aggressive
 * internal state for fast lookups is currently [in-progress](https://github.com/hashicorp/nomad-autoscaler/pull/30), this means the Nomad API will be put under load when running the autoscaler
 * there is currently a limited number of supported APMs, this will be addressed but limits the usability

The Nomad Autoscaler currently supports:
* **Horizontal Application Autoscaling**: The process of automatically controlling the number of instances of an application to have sufficient work throughput to meet service-level agreements (SLA). In Nomad, horizontal application autoscaling can be achieved by modifying the number of allocations in a task group based on the value of a relevant metric, such as CPU and memory utilization or number of open connections.

## Requirements

The autoscaler relies on Nomad APIs that were introduced in Nomad 0.11-beta1, some of which have been changed during the beta.
The compability requirements are as follows:

|                                     Autoscaler Version                                    | Nomad Version |
|:-----------------------------------------------------------------------------------------:|:-------------:|
| [0.0.1-techpreview1](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview1/) | 0.11.0-beta1  |
| [0.0.1-techpreview2](https://releases.hashicorp.com/nomad-autoscaler/0.0.1-techpreview2/) |    0.11.0     |
| [nightly](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly)             |    0.11.0     |

## Documentation
Documentation is available within this repository [here](./docs/README.md).

## Demo
The [Vagrant based demo](./demo/vagrant/README.md) provides a guided example of running and autoscaling an application based on Prometheus metrics using the Nomad Autoscaler.

## Building
The Nomad Autoscaler can be easily built for local testing or development using the `make build` command. This will output the compiled binary to `./bin/nomad-autoscaler`.

## Nightly Builds

As a tech preview, this project is under constant updates, so every day the [`nightly` release](https://github.com/hashicorp/nomad-autoscaler/releases/tag/nightly) is updated with binaries built off the latest code in the `master` branch. This should make it easier for you to try new features and bug fixes.
