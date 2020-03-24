# Vagrant Nomad Autoscaler Demo
This Vagrantfile and associated configuration and job files are meant to provide a basic demo for running and using the Nomad Autoscaler. The demo will enable autoscaling on an example web application, showing both scaling in and scaling out.

### Requirements
To use the Vagrant environment, first install Vagrant following these [instructions](https://www.vagrantup.com/docs/installation/). You will also need a virtualization tool, such as [VirtualBox](https://www.virtualbox.org/).

## Start the Virtual Machine
Vagrant is used to provision a virtual machine which will run a Nomad server and client for scheduling work. To create the virtual machine, you should run the `vagrant up` command.
```
$ vagrant up
```
This will take a few minutes as the base Ubuntu box must be downloaded and provisioned with both Docker and Nomad. Once this completes, you should see this output.
```
Bringing machine 'default' up with 'virtualbox' provider...
==> default: Importing base box 'ubuntu/bionic64'...
...
==> default: Running provisioner: deps (shell)...
```
At this point the Vagrant box is running and ready to go.

## Start the Nomad Base Jobs
In order to start the Nomad jobs you should ssh onto the vagrant machine using the `vagrant ssh` command and navigate to the jobs directory.

```
$ vagrant ssh
$ cd /home/vagrant/nomad-autoscaler/jobs/
```

The demo uses [HAProxy](https://www.haproxy.org/) to provide ingress services. The job also includes a Prometheus exporter, which allows HAProxy metrics to be exported for consumption via Prometheus. This should be started using the `nomad run` command.
```
$ nomad run haproxy.nomad
```

You can check the status of the job and the allocation using the `nomad status` command.
```
$ nomad status haproxy
$ nomad status <alloc-id>
```

Prometheus can now be started, again using the `nomad run` command. The virtual machine is set to forward a number of ports to your localhost, including the Prometheus UI. This can be seen in a web browser at http://127.0.0.1:9090.
```
$ nomad run prometheus.nomad
```

## Start the Autoscaler Component Jobs
First start the Nomad job which will be autoscaled. The `webapp.nomad` jobfile contains a scaling stanza which defines the key autoscaling parameters for a task group. There are a number of interesting a key options to understand.
- `enabled = false` is a parameter to allow operators to administratively disable scaling for a task group. 
- `source = "prometheus"` specifies that the Autoscaler will use the Prometheus APM plugin to retrieve metrics.
- `query  = "scalar(avg((haproxy..` is the query that will be run against the APM and is expected to return a single value.
- `strategy = { name = "target-value"` defines the calculation strategy the Autoscaler will use, in this case we a targeting a value.

Register the application to Nomad, so Nomad can start 3 allocations of the example web application.
```
$ nomad run webapp.nomad
```

The Autoscaler Nomad job can now be submitted. The Nomad Autoscaler does not require persistent state, but will store key data in-memory to reduce load on the Nomad API.
```
$ nomad run autoscaler.nomad
```
Check the logs of the Autoscaler to see that it has started up correctly and loaded all the plugins.
```
$ nomad logs -stderr <alloc-id>
```

## Enable the Scaling Policy and Scale Down
The submitted webapp job has a scaling stanza, but has scaling disabled. In order to enable the task group for scaling, you should edit the file and change the `enabled = false` line to read `enabled = true` within the scaling stanza. Once updated, preview what changes are to be made using the `nomad plan` command.
```
$ nomad plan webapp.nomad
```
Submit the updated version of the job, taking the index generated from the plan command.
```
$ nomad run -check-index <index-from-plan> webapp.nomad
```
The Autoscaler will now actively evaluate the example group of the webapp job, and will determine that the current count 3, is more than is needed to meet the required scaling target.
```
example log line
example log line
example log line
example log line
example log line
```
The Autoscaler will never scale a job group past either the min or max parameters. This ensures applications maintain high availability even during minimal load, while also not over scaling due to problems such as misconfiguration or faulty metrics values.

## Generate Load and Scale Up
In order to generate load, a tool called [hey](https://github.com/rakyll/hey) is installed and available on the virtual machine. It is recommended to create a second ssh connection to the virtual machine. Inside this terminal we can then generate load on the webapp service.
```
$ hey -z 1m -c 30 http://127.0.0.1:8080
```

The increase in load will be reflected through Prometheus metrics to the Autoscaler. Checking the Autoscaler logs, you should see messages indicating it has chosen to scale out the job due to the increase in load.
```
example log line
example log line
example log line
example log line
example log line
```

## Demo End
Congratulations, you have run through the Nomad Autoscaler Vagrant demo. In order to destroy the created virtual machine, close all SSH connection and then issue a `vagrant destroy -f` command.
```
$ vagrant destroy -f
```
