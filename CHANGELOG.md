## UNRELEASED

FEATURES:
 * __Datadog APM__: Datadog can be used as an APM source [[GH-241](https://github.com/hashicorp/nomad-autoscaler/pull/241)]
 * __Telemetry__: Initial telemetry implementation to emit key stats for monitoring [[GH-238](https://github.com/hashicorp/nomad-autoscaler/pull/238)]

IMPROVEMENTS:
 * cluster_scaling: allow Nomad client nodes to be optionally purged after termination [[GH-258](https://github.com/hashicorp/nomad-autoscaler/pull/258)]

BUG FIXES:
 * plugins: fix an issue which caused a failure to launch multiple plugins using the same driver [[GH-222](https://github.com/hashicorp/nomad-autoscaler/issues/222)]
 * policy: fix an issue where the Nomad Autoscaler would fail to canonicalize Nomad APM queries with non-default plugin name [[GH-216](https://github.com/hashicorp/nomad-autoscaler/issues/216)]

## 0.1.0 (July 09, 2020)

__BACKWARDS INCOMPATIBILITIES:__
 * policy: allow multiple `check`s in a policy [[GH-176](https://github.com/hashicorp/nomad-autoscaler/pull/176)]
 * agent: the `scan-interval` CLI flag and top-level `default_evaluation_interval` config option have been removed and replaced by `policy-default-evaluation-interval` and `policy.default_evaluation_interval` options respectively [[GH-197](https://github.com/hashicorp/nomad-autoscaler/pull/197)]

FEATURES:
 * __AWS ASG Horizontal Cluster Scaling__: Scale the number of Nomad clients within AWS AutoScaling groups [[GH-185](https://github.com/hashicorp/nomad-autoscaler/pull/185)]

IMPROVEMENTS:
 * agent: only enter out-of-bounds cooldown if time greater than 1s [[GH-139](https://github.com/hashicorp/nomad-autoscaler/pull/139)]
 * agent: scaling policies can now be loaded from a directory on local disk [[GH-178](https://github.com/hashicorp/nomad-autoscaler/pull/178)]
 * core: update Nomad API dependency to 0.12.0 [[GH-210](https://github.com/hashicorp/nomad-autoscaler/pull/210)]

BUG FIXES:
 * cli: fix incorrect flag help detail for `nomad-ca-path` [[GH-168](https://github.com/hashicorp/nomad-autoscaler/pull/168)]
 * policy/nomad: fix fast loop when Nomad policy syntax is incorrect [[GH-179](https://github.com/hashicorp/nomad-autoscaler/pull/179)]

## 0.0.2 (May 21, 2020)

__BACKWARDS INCOMPATIBILITIES:__
 * agent: configuration `scan_interval` renamed to `default_evaluation_interval` [[GH-114](https://github.com/hashicorp/nomad-autoscaler/pull/114)]

FEATURES:
 * agent: allow policies to specify `evaluation_interval` [[GH-30](https://github.com/hashicorp/nomad-autoscaler/pull/30)]
 * agent: allow policies to specify `cooldown` [[GH-117](https://github.com/hashicorp/nomad-autoscaler/pull/117)]

IMPROVEMENTS:
 * agent: use blocking queries to communicate with the Nomad API [[GH-38](https://github.com/hashicorp/nomad-autoscaler/issues/38)]
 * agent: improve command error output message when setting up agent [[GH-106](https://github.com/hashicorp/nomad-autoscaler/pull/106)]
 * agent: skip scaling action if the desired count matches the current count [[GH-108](https://github.com/hashicorp/nomad-autoscaler/pull/108)]
 * agent: the target-value strategy plugin is configured for launching as default [[GH-135](https://github.com/hashicorp/nomad-autoscaler/pull/135)]
 * cli: always use cli library exit code when exiting main function [[GH-130](https://github.com/hashicorp/nomad-autoscaler/pull/130)]
 * core: update Nomad API dependency to 0.11.2 [[GH-128](https://github.com/hashicorp/nomad-autoscaler/pull/128)]
 * plugins/apm/prometheus: use the logger rather than fmt.Print to output Prometheus query warnings [[GH-107](https://github.com/hashicorp/nomad-autoscaler/pull/107)]
 * plugins/strategy/target-value: add new policy configuration `precision` [[GH-132](https://github.com/hashicorp/nomad-autoscaler/issues/132)]

BUG FIXES:
 * agent: fix issue where Nomad Autoscaler would fail to re-connect to Nomad [[GH-119](https://github.com/hashicorp/nomad-autoscaler/issues/119)]
 * plugins/apm/nomad: fix Nomad APM bug when querying groups on multiple clients [[GH-125](https://github.com/hashicorp/nomad-autoscaler/pull/125)]
 * plugins/strategy: fix bug in external strategy plugins suggesting scale to zero [[GH-112](https://github.com/hashicorp/nomad-autoscaler/pull/122)]

## 0.0.1-techpreview2 (April 9, 2020)

IMPROVEMENTS:
 * core: update Nomad API dependency to 0.11.0 [[GH-85](https://github.com/hashicorp/nomad-autoscaler/pull/85)]
 * plugins: improve user experience when developing external plugins [[GH-82](https://github.com/hashicorp/nomad-autoscaler/pull/82)]
 * plugins/apm/prometheus: return an error when Prometheus APM query returns NaN [[GH-87](https://github.com/hashicorp/nomad-autoscaler/pull/87)]

BUG FIXES:
 * plugins/strategy/target-value: allow the internal `target-value` plugin to handle scaling to zero as well as use target values of zero [[GH-77](https://github.com/hashicorp/nomad-autoscaler/pull/77)]

## 0.0.1-techpreview1 (March 25, 2020)

Initial tech-preview release.
See https://github.com/hashicorp/nomad-autoscaler for documentation and known limitations.

REQUIREMENTS:
* Nomad 0.11-beta1 or later

FEATURES:
* Support for horizontal scaling of Nomad jobs.
* **APM plugins**: nomad and prometheus (built-in)
* **Strategy plugins**: target-value plugin (built-in)
* **Target plugins**: nomad task group count (built-in)
