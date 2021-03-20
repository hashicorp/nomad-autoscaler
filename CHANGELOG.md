## UNRELEASED

IMPROVEMENTS:
 * agent: Add CLI flags to configure policy evaluation [[GH-421](https://github.com/hashicorp/nomad-autoscaler/pull/421)]
 * agent (Enterprise): Add CLI flags to configure Dynamic Application Sizing [[GH-422](https://github.com/hashicorp/nomad-autoscaler/pull/422)]

BUG FIXES:
 * agent: Only allow querying Prometheus formatted metrics if Prometheus is enabled within the config [[GH-416](https://github.com/hashicorp/nomad-autoscaler/pull/416)]
 * agent: Fix an issue that could cause the agent to panic depending on the order configuration files were loaded [[GH-420](https://github.com/hashicorp/nomad-autoscaler/pull/420)]
 * policy: Prevent panic on policy monitoring [[GH-428](https://github.com/hashicorp/nomad-autoscaler/pull/428)]
 * policy: Ensure metric emitters use the correct context and are stopped when appropriate [[GH-408](https://github.com/hashicorp/nomad-autoscaler/pull/408)]

## 0.3.0 (February 25, 2021)

FEATURES:
  * __GCP MIG Horizontal Cluster Scaling__: Scale the number of Nomad clients within a GCP Managed Instance Groups [[GH-353](https://github.com/hashicorp/nomad-autoscaler/pull/353)]
  * agent: Add pprof HTTP debug endpoints [[GH-349](https://github.com/hashicorp/nomad-autoscaler/pull/349)]

IMPROVEMENTS:
 * agent: Update Nomad API dependency to v1.0.4 [[GH-401](https://github.com/hashicorp/nomad-autoscaler/pull/401)]
 * agent: Read Nomad address and region from environment variables [[GH-365](https://github.com/hashicorp/nomad-autoscaler/pull/365)]
 * helper/scaleutils: refactored scaleutils to remove burden on horizontal cluster scaling target developers and allow for external plugins without core changes [[GH-395](https://github.com/hashicorp/nomad-autoscaler/pull/395)]
 * plugins: Replace net/rpc plugin subsystem with gRPC implementation [[GH-355](https://github.com/hashicorp/nomad-autoscaler/pull/355)]
 * plugins/apm/prometheus: Update Prometheus client dependency from v1.5.1 to v1.9.0 [[GH-368](https://github.com/hashicorp/nomad-autoscaler/pull/368)]
 * plugins/target: Add cluster scaling configuration to ignore system jobs on drain [[GH-356](https://github.com/hashicorp/nomad-autoscaler/pull/356)]

BUG FIXES:
 * agent: Fix an issue where the Autoscaler could get blocked and stop evaluating policies [[GH-354](https://github.com/hashicorp/nomad-autoscaler/pull/354)]
 * agent: Fix Nomad config merging so that Nomad env vars are used correctly [[GH-381](https://github.com/hashicorp/nomad-autoscaler/pull/381)]
 * helper/scaleutils: Filter nodes to ensure unstable pools do not run evaluations [[GH-378](https://github.com/hashicorp/nomad-autoscaler/pull/378)]
 * plugins/target/aws-asg: Fix a bug where confirming instance termination would exit prematurely [[GH-392](https://github.com/hashicorp/nomad-autoscaler/pull/392)]

## 0.2.1 (January 12, 2021)

BUG FIXES:
 * plugins/apm: Fix a bug where external APM plugins would cause the Nomad Autoscaler to panic [[GH-341](https://github.com/hashicorp/nomad-autoscaler/pull/341)]

## 0.2.0 (January 06, 2021)

__BACKWARDS INCOMPATIBILITIES:__
 * apm/datadog: Queries should use the new `query_window` parameter [[GH-268](https://github.com/hashicorp/nomad-autoscaler/pull/268)]
 * policy/file: Policies stored in files must be wrapped in a `scaling` block [[GH-313](https://github.com/hashicorp/nomad-autoscaler/pull/313)]

FEATURES:
 * __Azure VMSS Horizontal Cluster Scaling__: Scale the number of Nomad clients within Azure virtual machine scale sets [[GH-278](https://github.com/hashicorp/nomad-autoscaler/pull/278)]
 * __Dynamic Application Sizing (Enterprise)__: Evaluate, processes and store historical task resource usage data, making recommendations for CPU and Memory resource parameters [[GH-298](https://github.com/hashicorp/nomad-autoscaler/pull/298)]

IMPROVEMENTS:
 * agent: Added new evaluation broker to manage storing, deduping and controlling the distribution policy evaluation requests to workers [[GH-282](https://github.com/hashicorp/nomad-autoscaler/pull/282)]
 * agent: Add `/v1/agent/reload` endpoint [[GH-312](https://github.com/hashicorp/nomad-autoscaler/pull/312)]
 * apm/nomad: CPU query relative to task group allocated resources [[GH-324](https://github.com/hashicorp/nomad-autoscaler/pull/324)]
 * apm/nomad: Memory query relative to task group allocated resources [[GH-334](https://github.com/hashicorp/nomad-autoscaler/pull/334)]
 * plugins/target/nomad: Added support for namespaced jobs [[GH-277](https://github.com/hashicorp/nomad-autoscaler/pull/277)]
 * policy: Add `query_window` parameter to `check` [[GH-268](https://github.com/hashicorp/nomad-autoscaler/pull/268)]
 * policy/file: Allow multiple policies per file [[GH-313](https://github.com/hashicorp/nomad-autoscaler/pull/313)]

## 0.1.1 (September 11, 2020)

FEATURES:
 * __Datadog APM__: Datadog can be used as an APM source [[GH-241](https://github.com/hashicorp/nomad-autoscaler/pull/241)]
 * __Telemetry__: Initial telemetry implementation to emit key stats for monitoring [[GH-238](https://github.com/hashicorp/nomad-autoscaler/pull/238)]

IMPROVEMENTS:
 * cluster_scaling: Allow Nomad client nodes to be optionally purged after termination [[GH-258](https://github.com/hashicorp/nomad-autoscaler/pull/258)]

BUG FIXES:
 * plugins: Fix an issue which caused a failure to launch multiple plugins using the same driver [[GH-222](https://github.com/hashicorp/nomad-autoscaler/issues/222)]
 * policy: Fix an issue where the Nomad Autoscaler would fail to canonicalize Nomad APM queries with non-default plugin name [[GH-216](https://github.com/hashicorp/nomad-autoscaler/issues/216)]

## 0.1.0 (July 09, 2020)

__BACKWARDS INCOMPATIBILITIES:__
 * policy: Allow multiple `check`s in a policy [[GH-176](https://github.com/hashicorp/nomad-autoscaler/pull/176)]
 * agent: The `scan-interval` CLI flag and top-level `default_evaluation_interval` config option have been removed and replaced by `policy-default-evaluation-interval` and `policy.default_evaluation_interval` options respectively [[GH-197](https://github.com/hashicorp/nomad-autoscaler/pull/197)]

FEATURES:
 * __AWS ASG Horizontal Cluster Scaling__: Scale the number of Nomad clients within AWS AutoScaling groups [[GH-185](https://github.com/hashicorp/nomad-autoscaler/pull/185)]

IMPROVEMENTS:
 * agent: Only enter out-of-bounds cooldown if time greater than 1s [[GH-139](https://github.com/hashicorp/nomad-autoscaler/pull/139)]
 * agent: Scaling policies can now be loaded from a directory on local disk [[GH-178](https://github.com/hashicorp/nomad-autoscaler/pull/178)]
 * core: Update Nomad API dependency to 0.12.0 [[GH-210](https://github.com/hashicorp/nomad-autoscaler/pull/210)]

BUG FIXES:
 * cli: Fix incorrect flag help detail for `nomad-ca-path` [[GH-168](https://github.com/hashicorp/nomad-autoscaler/pull/168)]
 * policy/nomad: Fix fast loop when Nomad policy syntax is incorrect [[GH-179](https://github.com/hashicorp/nomad-autoscaler/pull/179)]

## 0.0.2 (May 21, 2020)

__BACKWARDS INCOMPATIBILITIES:__
 * agent: Configuration `scan_interval` renamed to `default_evaluation_interval` [[GH-114](https://github.com/hashicorp/nomad-autoscaler/pull/114)]

FEATURES:
 * agent: Allow policies to specify `evaluation_interval` [[GH-30](https://github.com/hashicorp/nomad-autoscaler/pull/30)]
 * agent: Allow policies to specify `cooldown` [[GH-117](https://github.com/hashicorp/nomad-autoscaler/pull/117)]

IMPROVEMENTS:
 * agent: Use blocking queries to communicate with the Nomad API [[GH-38](https://github.com/hashicorp/nomad-autoscaler/issues/38)]
 * agent: Improve command error output message when setting up agent [[GH-106](https://github.com/hashicorp/nomad-autoscaler/pull/106)]
 * agent: Skip scaling action if the desired count matches the current count [[GH-108](https://github.com/hashicorp/nomad-autoscaler/pull/108)]
 * agent: The target-value strategy plugin is configured for launching as default [[GH-135](https://github.com/hashicorp/nomad-autoscaler/pull/135)]
 * cli: Always use cli library exit code when exiting main function [[GH-130](https://github.com/hashicorp/nomad-autoscaler/pull/130)]
 * core: Update Nomad API dependency to 0.11.2 [[GH-128](https://github.com/hashicorp/nomad-autoscaler/pull/128)]
 * plugins/apm/prometheus: Use the logger rather than fmt.Print to output Prometheus query warnings [[GH-107](https://github.com/hashicorp/nomad-autoscaler/pull/107)]
 * plugins/strategy/target-value: Add new policy configuration `precision` [[GH-132](https://github.com/hashicorp/nomad-autoscaler/issues/132)]

BUG FIXES:
 * agent: Fix issue where Nomad Autoscaler would fail to re-connect to Nomad [[GH-119](https://github.com/hashicorp/nomad-autoscaler/issues/119)]
 * plugins/apm/nomad: Fix Nomad APM bug when querying groups on multiple clients [[GH-125](https://github.com/hashicorp/nomad-autoscaler/pull/125)]
 * plugins/strategy: Fix bug in external strategy plugins suggesting scale to zero [[GH-112](https://github.com/hashicorp/nomad-autoscaler/pull/122)]

## 0.0.1-techpreview2 (April 9, 2020)

IMPROVEMENTS:
 * core: Update Nomad API dependency to 0.11.0 [[GH-85](https://github.com/hashicorp/nomad-autoscaler/pull/85)]
 * plugins: Improve user experience when developing external plugins [[GH-82](https://github.com/hashicorp/nomad-autoscaler/pull/82)]
 * plugins/apm/prometheus: Return an error when Prometheus APM query returns NaN [[GH-87](https://github.com/hashicorp/nomad-autoscaler/pull/87)]

BUG FIXES:
 * plugins/strategy/target-value: Allow the internal `target-value` plugin to handle scaling to zero as well as use target values of zero [[GH-77](https://github.com/hashicorp/nomad-autoscaler/pull/77)]

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
