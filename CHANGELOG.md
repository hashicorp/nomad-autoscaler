## UNRELEASED

IMPROVEMENTS:
* build: Updated to Go 1.22.1 [[GH-872](https://github.com/hashicorp/nomad-autoscaler/pull/872)]

## 0.4.2 (February 20, 2024)

IMPROVEMENTS:
 * plugin/strategy/target-value: Add new configuration `max_scale_up` and `max_scale_down` to allow restricting how much change is applied on each scaling event [[GH-848](https://github.com/hashicorp/nomad-autoscaler/pull/848)]
 * policy: Add new configuration `query_window_offset` to apply a time offset to the query window [[GH-850](https://github.com/hashicorp/nomad-autoscaler/pull/850)]

BUG FIXES:
 * agent: Fixed a bug that caused a target in dry-run mode to scale when outside of its min/max range [[GH-845](https://github.com/hashicorp/nomad-autoscaler/pull/845)]
 * agent: Fixed a bug that caused the Enterprise license checker to not be reloaded on `SIGHUP` [[GH-849](https://github.com/hashicorp/nomad-autoscaler/pull/849)]

## 0.4.1 (January 18, 2024)

IMPROVEMENTS:
 * agent: Add `nomad.block_query_wait_time` config option for Nomad API connectivity [[GH-755](https://github.com/hashicorp/nomad-autoscaler/pull/755)]
 * agent: Add `high_availability.lock_namespace` configuration to specify the namespace used for writing the high availability lock variable. [[GH-832](https://github.com/hashicorp/nomad-autoscaler/pull/832)]
 * build: Updated to Go 1.21.6 [[GH-831](https://github.com/hashicorp/nomad-autoscaler/pull/831)]
 * metrics: Add `policy_id` and `target_name` labels to `scale.invoke.success_count` and `scale.invoke.error_count` metrics [[GH-814](https://github.com/hashicorp/nomad-autoscaler/pull/814)]
 * plugin/target/aws: Add `scale_in_protection` configuration [[GH-807](https://github.com/hashicorp/nomad-autoscaler/pull/807)]
 * scaleutils: Add new node filter option `node_pool` to select nodes by their node pool value [[GH-810](https://github.com/hashicorp/nomad-autoscaler/pull/810)]

BUG FIXES:
 * agent: Fixed a bug that could cause the same scaling policy to be evaluated multiple times concurrently [[GH-812](https://github.com/hashicorp/nomad-autoscaler/pull/812)]
 * agent: Fixed a bug that caused the agent to panic when trying to evaluate a policy with a missing `check.strategy` block [[GH-813](https://github.com/hashicorp/nomad-autoscaler/pull/813)]
 * plugin/apm/nomad: Set correct namespace when querying group metrics [[GH-808](https://github.com/hashicorp/nomad-autoscaler/pull/808)]

## 0.4.0 (December 20, 2023)

FEATURES:
 * **High Availability**: Added support for high availability by allowing multiple instances of the autoscaler to run at the same time, but having only one actively executing [[GH-649](https://github.com/hashicorp/nomad-autoscaler/pull/649)]

IMPROVEMENTS:
 * build: Updated to Go 1.21.5 [[GH-790](https://github.com/hashicorp/nomad-autoscaler/pull/790)]
 * plugin/target/aws: Prevent scaling if an instance refresh is in progress [[GH-597](https://github.com/hashicorp/nomad-autoscaler/pull/597)]
 * plugin/target/aws: Add new configuration `retry_attempts` to account for potentially slow ASG update operations [[GH-594](https://github.com/hashicorp/nomad-autoscaler/pull/594)
 * agent: Update Nomad API dependency to v1.7.1 [[GH-796](https://github.com/hashicorp/nomad-autoscaler/pull/796)]
 * agent: Add specific metadata to drained nodes to allow for later identification [[GH-636](https://github.com/hashicorp/nomad-autoscaler/issues/627)]
 * agent: Logged 404s from jobs/policies going away lowered to debug [[GH-723](https://github.com/hashicorp/nomad-autoscaler/pull/723/)]
 * agent: Added config option to enable file and line log detail [[GH-769](https://github.com/hashicorp/nomad-autoscaler/pull/769/files)]

BUG FIXES:
 * plugin/apm/datadog: Fixed a panic when Datadog queries return `null` values [[GH-606](https://github.com/hashicorp/nomad-autoscaler/pull/606)]
 * agent: Re-start monitoring a job's scale info when a job is deleted then re-created [[GH-724](https://github.com/hashicorp/nomad-autoscaler/pull/724)]
 * agent: File policy sources now resume working after recovering from Nomad API errors [[GH-733](https://github.com/hashicorp/nomad-autoscaler/pull/733)]

## 0.3.7 (June 10, 2022)

IMPROVEMENTS:
 * agent: Scale target so it is within `min` and `max` values before evaluating the rest of the policy [[GH-588](https://github.com/hashicorp/nomad-autoscaler/pull/588)]
 * agent: Update `hashicorp/nomad/api` to v1.3.1 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * agent: Update `armon/go-metrics` to v0.3.11 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * build: Use `alpine:3.15` as base image and provide an entrypoint to run `nomad-autoscaler` by default [[GH-582](https://github.com/hashicorp/nomad-autoscaler/pull/582)]
 * build: Docker image with support for multiple architectures [[GH-582](https://github.com/hashicorp/nomad-autoscaler/pull/582)]
 * plugin/apm/datadog: Update Datadog client dependency to v1.14.0 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * plugin/apm/prometheus: Update Prometheus client dependency to v1.12.2 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * plugin/target/aws: Update AWS client dependency to v1.16.4 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * plugin/target/azure: Update Azure client dependency to v64.1.0 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]
 * plugin/target/gcp: Update GCP client dependency to 0.80.0 [[GH-585](https://github.com/hashicorp/nomad-autoscaler/pull/585)]

BUG FIXES:
 * plugin/target/aws: Fixed a regression issue that broke the default AWS credential chain [[GH-586](https://github.com/hashicorp/nomad-autoscaler/pull/586)]

## 0.3.6 (February 18, 2022)

IMPROVEMENTS:
 * plugins/target/aws-asg: Support EC2 role credentials [[GH-564](https://github.com/hashicorp/nomad-autoscaler/pull/564)]
 * policy: Add `group` to `check` configuration [[GH-567](https://github.com/hashicorp/nomad-autoscaler/pull/567)]
 * policy: Add `on_check_error` and `on_error` configuration [[GH-566](https://github.com/hashicorp/nomad-autoscaler/pull/566)]

## 0.3.5 (January 20, 2022)

IMPROVEMENTS:
 * build: Updated to Go 1.17.6 [[GH-556](https://github.com/hashicorp/nomad-autoscaler/pull/556)]

BUG FIXES:
 * plugins/apm/datadog: Log the correct rate limit error message [[GH-552](https://github.com/hashicorp/nomad-autoscaler/pull/552)]
 * plugins/target/nomad: Reload status handlers on SIGHUP [[GH-554](https://github.com/hashicorp/nomad-autoscaler/pull/554)]
 * policy: Fixed an issue that could cause a panic if the policy content is not read in time [[GH-551](https://github.com/hashicorp/nomad-autoscaler/pull/551)]

## 0.3.4 (November 24, 2021)

IMPROVEMENTS:
 * agent: Allow disabling specific policy sources [[GH-544](https://github.com/hashicorp/nomad-autoscaler/pull/544)]
 * agent: Dispense `fixed-value`, `pass-through`, and `threshold` strategy plugins by default [[GH-536](https://github.com/hashicorp/nomad-autoscaler/pull/536)]
 * build: Updated to Go 1.17 [[GH-545](https://github.com/hashicorp/nomad-autoscaler/pull/545)]
 * plugins/apm/datadog: Add support for custom server site [[GH-548](https://github.com/hashicorp/nomad-autoscaler/pull/548)]
 * plugins/apm/prometheus: Add support for basic auth and custom headers [[GH-522](https://github.com/hashicorp/nomad-autoscaler/pull/522)]
 * plugins/apm/prometheus: Add support for TLS CA certificates [[GH-547](https://github.com/hashicorp/nomad-autoscaler/pull/547)]
 * plugins/target/nomad: Reduce log level for active deployments error messages [[GH-542](https://github.com/hashicorp/nomad-autoscaler/pull/542)]
 * policy: Prevent scaling cluster to zero when using the Nomad APM [[GH-534](https://github.com/hashicorp/nomad-autoscaler/pull/534)]
 * scaleutils: Add combined filter to allow filtering by node class and datacenter [[GH-535](https://github.com/hashicorp/nomad-autoscaler/pull/535)]
 * scaleutils: Improve node selection on scale in actions to avoid errors due to invalid nodes [[GH-539](https://github.com/hashicorp/nomad-autoscaler/pull/539)]

BUG FIXES:
 * scaleutils: Fixed `least_busy` node selector on clusters running servers older than v1.0.0 [[GH-508](https://github.com/hashicorp/nomad-autoscaler/pull/508)]
 * plugins/strategy/threshold: Fixed an issue where the wrong scaling action was taken even when the threshold was no met [[GH-537](https://github.com/hashicorp/nomad-autoscaler/pull/537)]
 * plugins/target/nomad: Fixed an issue where a particular error message would not display the proper policy information [[GH-538](https://github.com/hashicorp/nomad-autoscaler/pull/538)]
 * policy: Prevent panic when a non-vertical scaling policy uses a DAS plugin [[GH-543](https://github.com/hashicorp/nomad-autoscaler/pull/543)]

## 0.3.3 (May 03, 2021)

FEATURES:
 * **Threshold Strategy**: A strategy plugin that allows for different scaling actions based on a set of tiers defined by upper and lower bound metric values [[GH-483](https://github.com/hashicorp/nomad-autoscaler/pull/483)]
 * plugins/target: Horizontal cluster scaling can now use the node `datacenter` parameter to group nodes [[GH-468](https://github.com/hashicorp/nomad-autoscaler/pull/468)]

BUG FIXES:
 * agent: Updated `hashicorp/nomad/api` to v1.1.0-beta to include several fixes [[GH-488](https://github.com/hashicorp/nomad-autoscaler/pull/488)]
 * agent: Updated `hashicorp/hcl/v2` to v2.10.0 to include several fixes [[GH-481](https://github.com/hashicorp/nomad-autoscaler/pull/481)]
 * agent: Updated `mitchellh/copystructure` to v1.1.2 to include several fixes [[GH-481](https://github.com/hashicorp/nomad-autoscaler/pull/481)]
 * agent: Updated `hashicorp/go-hclog` to v0.16.0 to include a fix to log rendering [[GH-481](https://github.com/hashicorp/nomad-autoscaler/pull/481)]
 * agent: Updated `armon/go-metrics` to v0.3.7 to include a fix to Prometheus metrics expiry [[GH-481](https://github.com/hashicorp/nomad-autoscaler/pull/481)]
 * plugins/target/aws-asg: Ensure user provided `aws_region` config option overrides AWS client region discovery [[GH-474](https://github.com/hashicorp/nomad-autoscaler/pull/474)]

## 0.3.2 (April 06, 2021)

FEATURES:
 * plugins/target: Add `empty_ignore_system` node selector strategy [[GH-450](https://github.com/hashicorp/nomad-autoscaler/pull/450)]

BUG FIXES:
 * plugins/target: Ensure only empty nodes are selected when using the `empty` node selector strategy [[GH-450](https://github.com/hashicorp/nomad-autoscaler/pull/450)]

## 0.3.1 (April 01, 2021)

FEATURES:
 * __Fixed Value Strategy__: A strategy plugin that scales to a constant configured value [[GH-436](https://github.com/hashicorp/nomad-autoscaler/pull/436)]
 * __Pass Through Strategy__: A strategy plugin uses the APM metric value as the output desired value [[GH-433](https://github.com/hashicorp/nomad-autoscaler/pull/433)]
 * plugins/target: Horizontal cluster scaling targets can now configure `node_selector_strategy` to control the process which identifies nodes for termination [[GH-435](https://github.com/hashicorp/nomad-autoscaler/pull/435)]

IMPROVEMENTS:
 * agent: Add CLI flags to configure policy evaluation [[GH-421](https://github.com/hashicorp/nomad-autoscaler/pull/421)]
 * agent (Enterprise): Add CLI flags to configure Dynamic Application Sizing [[GH-422](https://github.com/hashicorp/nomad-autoscaler/pull/422)]
 * plugins/target/aws-asg: Use single ASG call rather than split ASG/EC2 on scale-in [[GH-425](https://github.com/hashicorp/nomad-autoscaler/pull/425)]
 * policy: Make `query` optional inside `check`s with strategies that don't require an APM [[GH-442](https://github.com/hashicorp/nomad-autoscaler/pull/442)]

BUG FIXES:
 * agent: Updated `hashicorp/go-hclog` to v0.15.0 to include several fixes [[GH-434](https://github.com/hashicorp/nomad-autoscaler/pull/434)]
 * agent: Updated `hashicorp/hcl/v2` to v2.9.1 to include several panic fixes [[GH-434](https://github.com/hashicorp/nomad-autoscaler/pull/434)]
 * agent: Updated `mitchellh/cli` to v1.1.2 to include a fix to auto-complete [[GH-434](https://github.com/hashicorp/nomad-autoscaler/pull/434)]
 * agent: Updated `armon/go-metrics` to v0.3.6 to include a fix to the Prometheus sink [[GH-434](https://github.com/hashicorp/nomad-autoscaler/pull/434)]
 * agent: Only allow querying Prometheus formatted metrics if Prometheus is enabled within the config [[GH-416](https://github.com/hashicorp/nomad-autoscaler/pull/416)]
 * agent: Updated `hashicorp/go-multierror` to v1.1.1 to include a panic fix when using wrapped errors [[GH-434](https://github.com/hashicorp/nomad-autoscaler/pull/434)]
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
