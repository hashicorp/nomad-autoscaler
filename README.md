# Nomad Autoscaler [![Build Status](https://circleci.com/gh/hashicorp/nomad-autoscaler.svg?style=svg)](https://circleci.com/gh/hashicorp/nomad-autoscaler) [![Discuss](https://img.shields.io/badge/discuss-nomad-00BC7F?style=flat)](https://discuss.hashicorp.com/c/nomad)

***This project is in the early stages of development and is supplied without guarantees and subject to change without warning***

The Nomad Autoscaler is an autoscaling daemon for [Nomad](https://nomadproject.io/), architectured around plug-ins to allow for easy extensibility in terms of supported metrics sources, scaling targets and scaling algorithms.

The Nomad Autoscaler currently supports:
* **Horizontal Application Autoscaling**: The process of automatically controlling the number of instances of an application to have sufficient work throughput to meet service-level agreements (SLA).
