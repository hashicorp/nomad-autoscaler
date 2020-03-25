# Nomad Autoscaler Plugins
Plugins are an essential part of the Nomad Autoscaler architecture. The Autoscaler uses the [go-plugin](https://github.com/hashicorp/go-plugin) library to implement an ecosystem of different types of plugins. Each plugin type is responsible for a specific task; APM plugins retrieve metrics about the workloads being monitored and strategy plugins decide which actions Nomad should execute to keep the policy valid. The flexibility of plugins allows the Nomad Autoscaler to be extended to meet specific business requirements or technology use cases.

The Nomad Autoscaler currently ships with a number of built-in plugins to ease the learning curve. Details of these can be found below, under the specific plugin type sections.

## APM Plugins
APMs are used to store metrics about an applications performance and current state. The APM plugin is responsible for querying the APM and returning a value which will be used to determine if scaling should occur. Currently built into the Nomad Autoscaler binary are:

#### Prometheus APM Plugin
Use Prometheus metrics to scale your Nomad job task groups. The query performed on Prometheus should return a single value.
```hcl
policy {
  source = "prometheus"
  query  = "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))"
  ...
}
```

#### Nomad APM Plugin
Use Nomad CPU and Memory task group metrics for autoscaling.

## Target Plugins
Target Plugins determine where the resource to be autoscaled is located.

#### Nomad Target Plugin
This indicates the Nomad task group is running on a Nomad cluster. This block can be omitted as `nomad` is currently the default parameter.

## Strategy Plugins
Strategy plugins compare the current state of the system against the desired state defined by the operator in the scaling policy and generate an action that will bring the system closer to where it should be. In practical terms, strategies receive the current count and a metric value for a task group and output what the new task group count should be. Currently built into the Nomad Autoscaler binary are:

#### Target Value Strategy Plugin
The target value strategy plugin will perform count calculations in order to keep the value resulting from the APM query at or around a specified target.

An example would be attempting to keep the number of active connections to a web server at 20 per instance of the application.  
```hcl
strategy = {
  name = "target-value"

  config = {
    target = 20
  }
}
```
