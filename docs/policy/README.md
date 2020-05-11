# Nomad Autoscaler Scaling Policies
Nomad task groups can be configured for Autoscaling using the [scaling stanza](https://nomadproject.io/docs/job-specification/scaling/) in the job specification. The policy contents on the scaling stanza are opaque to Nomad, and are unique and parsed by the external autoscaler.

## General Options
 * `source` - The APM plugin that should handle the metric query. If omitted, this defaults to using the Nomad APM. 
 * `query` - The query to run against the specified APM. Currently this query should return a single value.
 * `evaluation_interval` - Defines how often the policy is evaluated by the Autoscaler. It should be provided as a duration (i.e.: `"5s"`, `"1m"` etc). If omitted the configuration value `scan_interval` from the agent will be used.
 * `cooldown` - A time interval after a scaling action during which no additional scaling will be performed on the resource. It should be provided as a duration (i.e.: `"5s"`, `"1m"` etc). If omitted the configuration value `policy.default_cooldown` from the agent will be used.
 * `target` - Defines where the autoscaling target is running. A Nomad task group for example has a target of Nomad, as it is running on a Nomad cluster. If omitted, this defaults to `name = "nomad"`. 
 * `strategy` - The strategy to use, and it's configuration when calculating the desired state based on the current task group count and the metric returned by the APM.

Below is a full Nomad task group scaling stanza example, including a valid policy for the Nomad Autoscaler.
```hcl
scaling {
  enabled = true
  min     = 1
  max     = 10

  policy {
    source = "prometheus"
    query  = "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))"

    evaluation_interval = "10s"
    cooldown            = "2m"

    strategy = {
      name = "target-value"

      config = {
        target = 20
      }
    }
  }
}
```
