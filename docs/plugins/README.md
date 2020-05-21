# Nomad Autoscaler Plugins
Plugins are an essential part of the Nomad Autoscaler architecture. The Autoscaler uses the [go-plugin](https://github.com/hashicorp/go-plugin) library to implement an ecosystem of different types of plugins. Each plugin type is responsible for a specific task; APM plugins retrieve metrics about the workloads being monitored and Strategy plugins decide which actions Nomad should execute to keep the policy valid. The flexibility of plugins allows the Nomad Autoscaler to be extended to meet specific business requirements or technology use cases.

The Nomad Autoscaler currently ships with a number of built-in plugins to ease the learning curve. Details of these can be found below, under the specific plugin type sections.

## APM Plugins
APMs are used to store metrics about an applications performance and current state. The APM (Application Performance Management) plugin is responsible for querying the APM and returning a value which will be used to determine if scaling should occur. Currently built into the Nomad Autoscaler binary are:

#### Prometheus APM Plugin
Use Prometheus metrics to scale your Nomad job task groups. The query performed on Prometheus should return a single value. You can use the [`scalar()`](https://prometheus.io/docs/prometheus/latest/querying/functions/#scalar) in your query to achieve this.

##### Sample policy

```hcl
policy {
  source = "prometheus"
  query  = "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))"
  ...
}
```

##### Sample agent configuration:

```hcl
apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "http://127.0.0.1:9090"
  }
}
```

##### Configuration

* `address` `string ("")` - Prometheus address in the `protocol://addr:port` format.

#### Nomad APM Plugin

Use Nomad CPU and memory task group metrics for autoscaling. In order to utilize this plugin, the configuration option [publish_allocation_metrics](https://nomadproject.io/docs/configuration/telemetry/#inlinecode-publish_allocation_metrics) should be set to `true` on the Nomad cluster. This is the default APM used by the Autoscaler if none is specified in the policy. It's also included in the agent, so no extra configuration is required.

Querying Nomad metrics can be done using the `operation_metric` syntax, where valid operations are:

* `avg` - returns the average of the metric value across allocations in the task group.
* `min` - returns the lowest metric value among the allocations in the task group.
* `max` - returns the highest metric value among the allocations in the task group.
* `sum` - returns the sum of all the metric values for the allocations in the task group.

The metric value can be:

* `cpu` - CPU usage as reported by the `nomad.client.allocs.cpu.total_percent` metric.
* `memory` - Memory usage as reported by the `nomad.client.allocs.memory.usage` metric.

##### Sample policy

```hcl
policy {
  source = "nomad"  # optinal, this is the default
  query  = "avg_cpu"
  ...
}
```

## Target Plugins
Target Plugins determine where the resource to be autoscaled is located.

#### Nomad Target Plugin
This indicates the Nomad task group is running on a Nomad cluster. This block can be omitted as `nomad` is currently the default parameter.

## Strategy Plugins
Strategy plugins compare the current state of the system against the desired state defined by the operator in the scaling policy and generate an action that will bring the system closer to the desired state. In practical terms, strategies receive the current count and a metric value for a task group and output what the new task group count should be. Currently built into the Nomad Autoscaler binary are:

#### Target Value Strategy Plugin
The target value strategy plugin will perform count calculations in order to keep the value resulting from the APM query at or around a specified target. The resulting count is calculated as a factor of the current number of allocations:

```
next_count = current_count * (metric_value / target)
```

##### Sample policy
An example would be attempting to keep the number of active connections to a web server at 20 per instance of the application.

```hcl
policy {
  ...
  strategy = {
    name = "target-value"

    config = {
      target    = 20
      precision = 0.0001
    }
  }
  ...
```

##### Policy configuration

* `target` `(float: <required>)` - Specifies the metric value the Autscaler should try to meet.
* `precision` `(float: 0.01)` - Specifies how significant a change in the input metric should be considered. Small precision values can lead to output fluctuation.
