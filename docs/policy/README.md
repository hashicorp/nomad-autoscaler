# Nomad Autoscaler Scaling Policies
Nomad task groups can be configured for Autoscaling using the [scaling stanza](https://nomadproject.io/docs/job-specification/scaling/) in the job specification. The policy contents are opaque to Nomad, and are unique and parsed by the external autoscaler.

## General Options
 * `source` - The APM plugin that should handle the metric query. If omitted, this defaults to using the Nomad APM. 
 * `query` - The query to run against the specified APM. Currently this query should return a single value.
 * `target` - Defines where the autoscaling target is running. A Nomad task group for example has a target of Nomad, as it is running on a Nomad cluster. If omitted, this defaults to `name = "nomad"`. 
 * `strategy` - The strategy to use, and it's configuration when calculating the desired state based on the current task group count and the metric returned by the APM.
 
### Full Example
```hcl
policy {
  source = "prometheus"
  query  = "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))"

  target = {
    name = "nomad"
  }

  strategy = {
    name = "target-value"

    config = {
      target = 20
    }
  }
}
```
