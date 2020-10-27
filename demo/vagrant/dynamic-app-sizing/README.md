# Dynamic Application Sizing demo
This Vagrantfile and associated configuration and job files provide a basic demo for running and
using the Dynamic Application Sizing feature of Nomad Enterprise and Nomad Autoscaler Enterprise.
The demo will enable Dynamic Application Sizing on an example web application.

## Requirements
To use the Vagrant environment, first install Vagrant following these [instructions](https://www.vagrantup.com/docs/installation/).
You will also need a virtualization tool, such as [VirtualBox](https://www.virtualbox.org/).

## Start the Virtual Machine
Vagrant is used to provision a virtual machine which will run a Nomad server and client for scheduling
work. To create the virtual machine, you should run the `vagrant up` command.
```shellsession
$ vagrant up
```
This will take a few minutes as the base Ubuntu box must be downloaded and provisioned with both Docker
and Nomad. Once this completes, you should see this output.
```shellsession
Bringing machine 'default' up with 'virtualbox' provider...
==> default: Importing base box 'ubuntu/bionic64'...
...
==> default: Running provisioner: deps (shell)...
```
At this point the Vagrant box is running and ready to go.

## Start the Nomad Base Jobs
In order to start the Nomad jobs you should ssh onto the vagrant machine using the `vagrant ssh`
command and navigate to the jobs directory.
```shellsession
$ vagrant ssh
$ cd /home/vagrant/nomad-autoscaler/jobs/
```

The demo uses [Prometheus](https://prometheus.io/) to provide historical metrics on Nomad job resources.
This should be started using the `nomad run` command. Once the application has started, the Prometheus
UI can be viewed from your workstation by browsing to http://127.0.0.1:9090.
```shellsession
$ nomad run prometheus.nomad
```

You can check the status of the job and the allocation using the `nomad status` command.
```shellsession
$ nomad status prometheus
$ nomad status <alloc-id>
```

The example application can now be started, again using the `nomad run` command. The job specification
includes scaling policies within the task stanza, which enables the task for CPU and Memory based
dynamic sizing.
```shellsession
$ nomad run example.nomad
```

With Prometheus and the example job running, you can view the raw data used by the Nomad Autoscaler
to make recommendations at any time at the following endpoints:
* [allocation CPU resource usage](http://localhost:9090/graph?g0.range_input=30m&g0.expr=nomad_client_allocs_cpu_total_ticks%7Bexported_job%3D%22example%22%2Ctask%3D%22redis%22%2Ctask_group%3D%22cache%22%7D&g0.tab=0)
* [allocation memory resource usage](http://localhost:9090/graph?g0.range_input=30m&g0.expr=nomad_client_allocs_memory_usage%7Bexported_job%3D%22example%22%2Ctask%3D%22redis%22%2Ctask_group%3D%22cache%22%7D%2F1024%2F1024&g0.tab=0)

## Start the Nomad Autoscaler
The Nomad Autoscaler Enterprise binary is available on the Vagrant machine. In order to run the binary
you can use the provided `das-autoscaler.nomad` job.
```shellsession
$ nomad run das-autoscaler.nomad
```

To see what's the Autoscaler is doing, you can tail its logs in the [Nomad UI](http://localhost:4646)
or via the CLI with the command:
```shellsession
$ nomad alloc logs -f -stderr <alloc_id>
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=nomad-target
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=app-sizing-nomad
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=nomad-apm
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=prometheus
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=target-value
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=app-sizing-avg
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=app-sizing-max
2020-10-19T22:37:11.531Z [INFO]  agent.plugin_manager: successfully launched and dispensed plugin: plugin_name=app-sizing-percentile
2020-10-19T22:37:11.533Z [INFO]  agent.http_server: server now listening for connections: address=127.0.0.1:8080
2020-10-19T22:37:11.533Z [DEBUG] nomad_policy_source: starting policy blocking query watcher
2020-10-19T22:37:11.533Z [INFO]  policy_eval.worker: starting worker: id=6af2f4b3-52d8-1466-eda0-c47516e5396a queue=vertical_mem
2020-10-19T22:37:11.534Z [DEBUG] policy_eval.broker: dequeue eval: queue=vertical_mem
2020-10-19T22:37:11.534Z [DEBUG] policy_eval.broker: waiting for eval: queue=vertical_mem
2020-10-19T22:37:11.534Z [INFO]  policy_eval.worker: starting worker: id=4e4c01d8-da8a-9a6c-59ca-bb524691ec14 queue=vertical_cpu
...
```

## Check for Dynamic Application Sizing Recommendations
Once the Nomad Autoscaler is running, it will take about 10 seconds until the first recommendation is
calculated and submitted to Nomad. You can navigate to the [Nomad Optimize](http://localhost:4646/ui/optimize)
UI page to view this. Feel free to accept or reject them.

### Generate Application Load
You can cause a load increase on the example application by using the `redis-benchmark` CLI to interact
with the Redis instances.

This CLI is wrapped in a Nomad dispatch batch job. First, register the batch job in Nomad:
```shellsession
$ nomad run das-load-test.nomad
Job registration successful
```

And then run it with the command:
```shellsession
$ nomad job dispatch das-load-test
Dispatched Job ID = das-load-test/dispatch-1603150078-4ce64dd2
Evaluation ID     = 003c42fb

==> Monitoring evaluation "003c42fb"
    Evaluation triggered by job "das-load-test/dispatch-1603150078-4ce64dd2"
    Allocation "1e6c9ae1" created: node "96dfe679", group "redis-benchmark"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "003c42fb" finished with status "complete"
```

The increase in load should result in new recommendation being availble within Nomad. You can again
navigate to the [Nomad Optimize](http://localhost:4646/ui/optimize) page to view and interact with
them.

If you leave the jobs running with no extra load, you will receive recommendations that slowly
decrease resource limits since there's no more load in the system.

## Demo End
Congratulations, you have run through the Dynamic Application Sizing Nomad Autoscaler Vagrant demo.
In order to destroy the created virtual machine, close all SSH connection and then issue a `vagrant destroy -f`
command.
```
$ vagrant destroy -f
```
