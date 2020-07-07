# Full Horizontal Autoscaling Demo
The demo resources contained within this directory provide a basic demo for running full horizontal
application and cluster autoscaling using the Nomad Autoscaler. The application will scale based on
the average number of connections per instance of our application. The cluster will scale based on
the total allocated resources across our pool of Nomad client nodes.

***The infrastructure built as part of the demo has billable costs and is not suitable for
production use**

### Requirements
In order to build and run the demo, the following applications are required locally:
 * HashiCorp Packer [1.5.6](https://releases.hashicorp.com/packer/1.5.6/)
 * HashiCorp Terraform [0.12.23](https://releases.hashicorp.com/terraform/0.12.23/)
 * rakyll/hey [latest](https://github.com/rakyll/hey#installation)

## Infrastructure Build
There are specific steps to build the infrastructure depending on which provider you wish to use.
Please navigate to the appropriate section below.
 * [Amazon Web Services](./aws.md)

## The Demo
The steps below this point are generic across providers and form the main part of this demo. Enjoy.

## Generate Application Load
In order to generate some initial load we will call the hey application. This will cause the
application to scale up slightly.
```
hey -z 10m -c 20 -q 40 $NOMAD_CLIENT_DNS:80 &
```

Viewing the autoscaler logs or the Grafana dashboard should show the application count increase
from `1` to `2`. Once this scaling has taken place, you can trigger additional load on the app that
causes further scaling.
```
hey -z 10m -c 20 -q 40 $NOMAD_CLIENT_DNS:80 &
```

This will again causes the application to scale which in-turn reduces the available resources on
our cluster. The reduction is such that the Autoscaler will decide a cluster scaling action is
required and trigger the appropriate action.
```
2020-07-06T08:44:46.460Z [INFO]  agent.worker.check_handler: received policy check for evaluation: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target
2020-07-06T08:44:46.460Z [INFO]  agent.worker.check_handler: fetching current count: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target
2020-07-06T08:44:46.460Z [INFO]  agent.worker.check_handler: querying source: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target query=scalar(sum(traefik_entrypoint_open_connections{entrypoint="webapp"})/scalar(nomad_nomad_job_summary_running{task_group="demo"}))
2020-07-06T08:44:46.468Z [INFO]  agent.worker.check_handler: calculating new count: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target count=1 metric=19
2020-07-06T08:44:46.469Z [INFO]  agent.worker.check_handler: scaling target: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target from=1 to=2 reason="scaling up because factor is 1.900000" meta=map[]
2020-07-06T08:44:46.487Z [INFO]  agent.worker.check_handler: successfully submitted scaling action to target: check=avg_sessions policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 source=prometheus strategy=target-value target=nomad-target desired_count=2
2020-07-06T08:44:46.487Z [INFO]  agent.worker: policy evaluation complete: policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 target=nomad-target
2020-07-06T08:44:46.487Z [DEBUG] policy_manager.policy_handler: scaling policy has been placed into cooldown: policy_id=6a60613b-be07-ba2d-1170-63237c5bb454 cooldown=1m0s
```

The Nomad Autoscaler logs will detail the action which can also be viewed via the Grafana dashboard
or the provide UI.
```
2020-07-06T08:46:16.541Z [INFO]  agent.worker: received policy for evaluation: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 target=aws-asg
2020-07-06T08:46:16.542Z [INFO]  agent.worker.check_handler: received policy check for evaluation: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:46:16.542Z [INFO]  agent.worker.check_handler: fetching current count: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:46:16.542Z [INFO]  agent.worker.check_handler: received policy check for evaluation: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:46:16.542Z [INFO]  agent.worker.check_handler: fetching current count: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:46:16.683Z [INFO]  agent.worker.check_handler: querying source: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg query=scalar(sum(nomad_client_allocated_cpu{node_class="hashistack"}*100/(nomad_client_unallocated_cpu{node_class="hashistack"}+nomad_client_allocated_cpu{node_class="hashistack"}))/count(nomad_client_allocated_cpu))
2020-07-06T08:46:16.685Z [INFO]  agent.worker.check_handler: calculating new count: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg count=1 metric=57
2020-07-06T08:46:16.685Z [INFO]  agent.worker.check_handler: nothing to do: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:46:16.699Z [INFO]  agent.worker.check_handler: querying source: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg query=scalar(sum(nomad_client_allocated_memory{node_class="hashistack"}*100/(nomad_client_unallocated_memory{node_class="hashistack"}+nomad_client_allocated_memory{node_class="hashistack"}))/count(nomad_client_allocated_memory))
2020-07-06T08:46:16.700Z [INFO]  agent.worker.check_handler: calculating new count: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg count=1 metric=95.17948717948718
2020-07-06T08:46:16.700Z [INFO]  agent.worker.check_handler: scaling target: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg from=1 to=2 reason="scaling up because factor is 1.359707" meta=map[]
2020-07-06T08:46:27.131Z [INFO]  internal_plugin.aws-asg: successfully performed and verified scaling out: action=scale_out asg_name=hashistack-nomad_client desired_count=2
2020-07-06T08:46:27.131Z [INFO]  agent.worker.check_handler: successfully submitted scaling action to target: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg desired_count=2
2020-07-06T08:46:27.131Z [INFO]  agent.worker: policy evaluation complete: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 target=aws-asg
2020-07-06T08:46:27.131Z [DEBUG] policy_manager.policy_handler: scaling policy has been placed into cooldown: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 cooldown=2m0s
```

## Remove Application Load
We can now simulate a reduction in load on the application by killing the running `hey` processes.
```
pkill hey
```

The reduction in load will cause the Autoscaler to firstly scale in the taskgroup. Once the
taskgroup has scaled in a sufficient amount, the Autoscaler will scale in the cluster. It
performs this work by selecting a node to remove, draining the node of all work and then
terminating it within the provider.
```
2020-07-06T08:50:16.648Z [INFO]  agent.worker: received policy for evaluation: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 target=aws-asg
2020-07-06T08:50:16.648Z [INFO]  agent.worker.check_handler: received policy check for evaluation: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:50:16.648Z [INFO]  agent.worker.check_handler: fetching current count: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:50:16.648Z [INFO]  agent.worker.check_handler: received policy check for evaluation: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:50:16.648Z [INFO]  agent.worker.check_handler: fetching current count: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg
2020-07-06T08:50:16.791Z [INFO]  agent.worker.check_handler: querying source: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg query=scalar(sum(nomad_client_allocated_memory{node_class="hashistack"}*100/(nomad_client_unallocated_memory{node_class="hashistack"}+nomad_client_allocated_memory{node_class="hashistack"}))/count(nomad_client_allocated_memory))
2020-07-06T08:50:16.795Z [INFO]  agent.worker.check_handler: calculating new count: check=mem_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg count=2 metric=32
2020-07-06T08:50:16.824Z [INFO]  agent.worker.check_handler: querying source: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg query=scalar(sum(nomad_client_allocated_cpu{node_class="hashistack"}*100/(nomad_client_unallocated_cpu{node_class="hashistack"}+nomad_client_allocated_cpu{node_class="hashistack"}))/count(nomad_client_allocated_cpu))
2020-07-06T08:50:16.825Z [INFO]  agent.worker.check_handler: calculating new count: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg count=2 metric=12.5
2020-07-06T08:50:16.825Z [INFO]  agent.worker.check_handler: scaling target: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg from=2 to=1 reason="scaling down because factor is 0.178571" meta=map[]
2020-07-06T08:50:16.896Z [DEBUG] internal_plugin.aws-asg: filtering node list: filter=class value=hashistack
2020-07-06T08:50:16.896Z [DEBUG] internal_plugin.aws-asg: identified Nomad node for removal: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b
2020-07-06T08:50:16.899Z [DEBUG] internal_plugin.aws-asg: identified remote provider ID for node: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b remote_id=i-0e1fc1328d1aebafa
2020-07-06T08:50:16.899Z [INFO]  internal_plugin.aws-asg: triggering drain on node: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b deadline=5m0s
2020-07-06T08:50:16.907Z [INFO]  internal_plugin.aws-asg: received node drain message: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b msg="Drain complete for node 1ae4318e-0f4e-b01e-0107-6082f3b6199b"
2020-07-06T08:50:16.909Z [DEBUG] internal_plugin.aws-asg: received node drain message: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b msg="Alloc "ef65727c-2361-e6f9-c23a-cc97a0a940ae" draining"
2020-07-06T08:50:22.260Z [DEBUG] internal_plugin.aws-asg: received node drain message: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b msg="Alloc "ef65727c-2361-e6f9-c23a-cc97a0a940ae" status running -> complete"
2020-07-06T08:50:22.260Z [INFO]  internal_plugin.aws-asg: received node drain message: node_id=1ae4318e-0f4e-b01e-0107-6082f3b6199b msg="All allocations on node "1ae4318e-0f4e-b01e-0107-6082f3b6199b" have stopped"
2020-07-06T08:50:22.405Z [DEBUG] internal_plugin.aws-asg: detaching instances from AutoScaling Group: action=scale_in asg_name=hashistack-nomad_client instances=[i-0e1fc1328d1aebafa]
2020-07-06T08:50:32.803Z [INFO]  internal_plugin.aws-asg: successfully detached instances from AutoScaling Group: action=scale_in asg_name=hashistack-nomad_client instances=[i-0e1fc1328d1aebafa]
2020-07-06T08:50:32.936Z [DEBUG] internal_plugin.aws-asg: terminating EC2 instances: action=scale_in asg_name=hashistack-nomad_client instances=[i-0e1fc1328d1aebafa]
2020-07-06T08:50:33.098Z [INFO]  internal_plugin.aws-asg: successfully terminated EC2 instances: action=scale_in asg_name=hashistack-nomad_client instances=[i-0e1fc1328d1aebafa]
2020-07-06T08:50:33.247Z [INFO]  agent.worker.check_handler: successfully submitted scaling action to target: check=cpu_allocated_percentage policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 source=prometheus strategy=target-value target=aws-asg desired_count=1
2020-07-06T08:50:33.247Z [INFO]  agent.worker: policy evaluation complete: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 target=aws-asg
2020-07-06T08:50:33.247Z [DEBUG] policy_manager.policy_handler: scaling policy has been placed into cooldown: policy_id=bf68649a-d087-2e69-362e-bbe71b5544f7 cooldown=2m0s
```

## Destroy the Infrastructure
It is important to destroy the created infrastructure as soon as you are finished with the demo. In
order to do this you should navigate to your Terraform env directory and issue a `destroy` command.
```
$ cd terraform/env/<env>
$ terraform destroy --auto-approve
```

Please also check and complete any provider specific steps:
 * [Amazon Web Services](./aws.md#post-demo-steps)
