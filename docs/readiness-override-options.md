# Readiness Override Options (Supported with Caveats)

The autoscaler supports the following readiness override options for cluster scaling flows:

- `node_filter_ignore_drain`
- `node_filter_ignore_init`
- `ignore_asg_events`

These options are supported, but they trade strict readiness behavior for faster progress during node transitions.

## Risks to consider

- potential scaling oscillation
- temporary capacity overshoot/undershoot while transitions are still converging
- reduced accuracy of “stable cluster” assumptions during evaluation windows

## When to use

- spot-heavy clusters with frequent replacement events
- environments with long drain windows (for example, long-running jobs)
- workloads where meeting SLA and restoring headroom quickly is prioritized

## When not to use

- clusters prioritizing strict stability and conservative scaling behavior
- environments where capacity accounting accuracy is more important than scaling responsiveness
- workloads sensitive to short-term over/under scaling during transitions
