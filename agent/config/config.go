// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/file"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/mitchellh/copystructure"
)

// Agent is the overall configuration of an autoscaler agent and includes all
// required information for it to start successfully.
//
// All time.Duration values should have two parts:
//   - a string field tagged with an hcl:"foo" and json:"-"
//   - a time.Duration field in the same struct which is populated within the
//     parseFile if the HCL param is populated.
//
// The string reference of a duration can include "ns", "us" (or "µs"), "ms",
// "s", "m", "h" suffixes.
type Agent struct {

	// LogLevel is the level of the logs to emit.
	LogLevel string `hcl:"log_level,optional"`

	// LogJson enables log output in JSON format.
	LogJson bool `hcl:"log_json,optional"`

	// EnableDebug is used to enable debugging HTTP endpoints.
	EnableDebug bool `hcl:"enable_debug,optional"`

	// PluginDir is the directory that holds the autoscaler plugin binaries.
	PluginDir string `hcl:"plugin_dir,optional"`

	// DynamicApplicationSizing is the configuration for the components used
	// in Dynamic Application Sizing.
	DynamicApplicationSizing *DynamicApplicationSizing `hcl:"dynamic_application_sizing,block" modes:"ent"`

	// HTTP is the configuration used to setup the HTTP health server.
	HTTP *HTTP `hcl:"http,block"`

	// Nomad is the configuration used to setup the Nomad client.
	Nomad *Nomad `hcl:"nomad,block"`

	// Policy is the configuration used to setup the policy manager.
	Policy *Policy `hcl:"policy,block"`

	// PolicyWorkers is the configuration used to define the number of workers
	// to start for each policy type.
	PolicyEval *PolicyEval `hcl:"policy_eval,block"`

	// Telemetry is the configuration used to setup metrics collection.
	Telemetry *Telemetry `hcl:"telemetry,block"`

	// HighAvailability is the configuration used for the leader election.
	HighAvailability *HighAvailability `hcl:"high_availability,block"`

	APMs       []*Plugin `hcl:"apm,block"`
	Targets    []*Plugin `hcl:"target,block"`
	Strategies []*Plugin `hcl:"strategy,block"`
}

// DynamicApplicationSizing contains configuration values to control the
// components used for Dynamic Application Sizing.
type DynamicApplicationSizing struct {

	// MetricsPreloadThreshold is the limit for how much historical data to
	// preload when the Autoscaler starts.
	MetricsPreloadThreshold    time.Duration
	MetricsPreloadThresholdHCL string `hcl:"metrics_preload_threshold,optional" json:"-"`

	// EvaluateAfter is the time limit for how much historical data must be
	// available before the Autoscaler evaluates a policy.
	EvaluateAfter    time.Duration
	EvaluateAfterHCL string `hcl:"evaluate_after,optional" json:"-"`

	// NamespaceLabel is the label used by the APM to store the namespace of a job.
	NamespaceLabel string `hcl:"namespace_label,optional"`

	// JobLabel is the label used by the APM to store the ID of a job.
	JobLabel string `hcl:"job_label,optional"`

	// GroupLabel is the label used by the APM to store the name of a group.
	GroupLabel string `hcl:"group_label,optional"`

	// TaskLabel is the label used by the APM to store the name of a task.
	TaskLabel string `hcl:"task_label,optional"`

	// CPUMetric is the metric used to query historical CPU usage.
	CPUMetric string `hcl:"cpu_metric,optional"`

	// MemoryMetric is the metric used to query historical memory usage.
	MemoryMetric string `hcl:"memory_metric,optional"`
}

// HTTP contains all configuration details for the running of the agent HTTP
// health server.
type HTTP struct {

	// BindAddress is the tcp address to bind to.
	BindAddress string `hcl:"bind_address,optional"`

	// BindPort is the port used to run the HTTP server.
	BindPort int `hcl:"bind_port,optional"`
}

// Nomad holds the user specified configuration for connectivity to the Nomad
// API.
type Nomad struct {

	// Address is the address of the Nomad agent.
	Address string `hcl:"address,optional"`

	// Region to use.
	Region string `hcl:"region,optional"`

	// Namespace to use.
	Namespace string `hcl:"namespace,optional"`

	// Token is the SecretID of an ACL token to use to authenticate API
	// requests with.
	Token string `hcl:"token,optional"`

	// HTTPAuth is the auth info to use for http access.
	HTTPAuth string `hcl:"http_auth,optional"`

	// CACert is the path to a PEM-encoded CA cert file to use to verify the
	// Nomad server SSL certificate.
	CACert string `hcl:"ca_cert,optional"`

	// CAPath is the path to a directory of PEM-encoded CA cert files to verify
	// the Nomad server SSL certificate.
	CAPath string `hcl:"ca_path,optional"`

	// ClientCert is the path to the certificate for Nomad communication.
	ClientCert string `hcl:"client_cert,optional"`

	// ClientKey is the path to the private key for Nomad communication.
	ClientKey string `hcl:"client_key,optional"`

	// TLSServerName, if set, is used to set the SNI host when connecting via
	// TLS.
	TLSServerName string `hcl:"tls_server_name,optional"`

	// SkipVerify enables or disables SSL verification.
	SkipVerify bool `hcl:"skip_verify,optional"`
}

// Telemetry holds the user specified configuration for metrics collection.
type Telemetry struct {

	// PrometheusRetentionTime is the retention time for prometheus metrics if
	// greater than 0.
	PrometheusRetentionTime    time.Duration
	PrometheusRetentionTimeHCL string `hcl:"prometheus_retention_time,optional" json:"-"`

	// PrometheusMetrics specifies whether the agent should make Prometheus
	// formatted metrics available.
	PrometheusMetrics bool `hcl:"prometheus_metrics,optional"`

	// DisableHostname specifies if gauge values should be prefixed with the
	// local hostname.
	DisableHostname bool `hcl:"disable_hostname,optional"`

	// EnableHostnameLabel adds the hostname as a label on all metrics.
	EnableHostnameLabel bool `hcl:"enable_hostname_label,optional"`

	// CollectionInterval specifies the time interval at which the agent
	// collects telemetry data.
	CollectionInterval    time.Duration
	CollectionIntervalHCL string `hcl:"collection_interval,optional" json:"-"`

	// StatsiteAddr specifies the address of a statsite server to forward
	// metrics data to.
	StatsiteAddr string `hcl:"statsite_address,optional"`

	// StatsdAddr specifies the address of a statsd server to forward metrics
	// to.
	StatsdAddr string `hcl:"statsd_address,optional"`

	// DogStatsDAddr specifies the address of a DataDog statsd server to
	// forward metrics to.
	DogStatsDAddr string `hcl:"dogstatsd_address,optional"`

	// DogStatsDTags specifies a list of global tags that will be added to all
	// telemetry packets sent to DogStatsD.
	DogStatsDTags []string `hcl:"dogstatsd_tags,optional"`

	// Circonus: see https://github.com/circonus-labs/circonus-gometrics
	// for more details on the various configuration options.

	// CirconusAPIToken is a valid API Token used to create/manage check. If
	// provided, metric management is enabled. Defaults to none.
	CirconusAPIToken string `hcl:"circonus_api_token,optional"`

	// CirconusAPIApp is an app name associated with API token. Defaults to
	// "nomad_autoscaler".
	CirconusAPIApp string `hcl:"circonus_api_app,optional"`

	// CirconusAPIURL is the base URL to use for contacting the Circonus API.
	// Defaults to "https://api.circonus.com/v2".
	CirconusAPIURL string `hcl:"circonus_api_url,optional"`

	// CirconusSubmissionInterval is the interval at which metrics are
	// submitted to Circonus. Defaults to 10s.
	CirconusSubmissionInterval string `hcl:"circonus_submission_interval,optional"`

	// CirconusCheckSubmissionURL is the check.config.submission_url field from
	// a previously created HTTPTRAP check. Defaults to none.
	CirconusCheckSubmissionURL string `hcl:"circonus_submission_url,optional"`

	// CirconusCheckID is the check id (not check bundle id) from a previously
	// created HTTPTRAP check. The numeric portion of the check._cid field.
	// Defaults to none.
	CirconusCheckID string `hcl:"circonus_check_id,optional"`

	// CirconusCheckForceMetricActivation will force enabling metrics, as they
	// are encountered, if the metric already exists and is NOT active. If
	// check management is enabled, the default behavior is to add new metrics
	// as they are encountered. If the metric already exists in the check, it
	// will *NOT* be activated. This setting overrides that behavior. Defaults
	// to "false".
	CirconusCheckForceMetricActivation string `hcl:"circonus_check_force_metric_activation,optional"`

	// CirconusCheckInstanceID serves to uniquely identify the metrics coming
	// from this "instance". It can be used to maintain metric continuity with
	// transient or ephemeral instances as they move around within an
	// infrastructure. Defaults to hostname:app.
	CirconusCheckInstanceID string `hcl:"circonus_check_instance_id,optional"`

	// CirconusCheckSearchTag is a special tag which, when coupled with the
	// instance id, helps to narrow down the search results when neither a
	// Submission URL or Check ID is provided. Defaults to service:app.
	CirconusCheckSearchTag string `hcl:"circonus_check_search_tag,optional"`

	// CirconusCheckTags is a comma separated list of tags to apply to the
	// check. Note that the value of CirconusCheckSearchTag will always be
	// added to the check. Defaults to none.
	CirconusCheckTags string `hcl:"circonus_check_tags,optional"`

	// CirconusCheckDisplayName is the name for the check which will be
	// displayed in the Circonus UI. Defaults to the value of
	// CirconusCheckInstanceID.
	CirconusCheckDisplayName string `hcl:"circonus_check_display_name,optional"`

	// CirconusBrokerID is an explicit broker to use when creating a new check.
	// The numeric portion of broker._cid. If metric management is enabled and
	// neither a Submission URL nor Check ID is provided, an attempt will be
	// made to search for an existing check using Instance ID and Search Tag.
	// If one is not found, a new HTTPTRAP check will be created. Default: use
	// Select Tag if provided, otherwise, a random Enterprise Broker associated
	// with the specified API token or the default Circonus Broker. Defaults to
	// none.
	CirconusBrokerID string `hcl:"circonus_broker_id,optional"`

	// CirconusBrokerSelectTag is a special tag which will be used to select a
	// broker when a Broker ID is not provided. The best use of this is to as a
	// hint for which broker should be used based on *where* this particular
	// instance is running. (e.g. a specific geo location or datacenter, dc:sfo)
	// Defaults to none.
	CirconusBrokerSelectTag string `hcl:"circonus_broker_select_tag,optional"`
}

type HighAvailability struct {
	Enable   bool   `hcl:"high_availability"`
	LockPath string `hcl:"lock_path,optional"  json:"-"`
}

// Plugin is an individual configured plugin and holds all the required params
// to successfully dispense the driver.
type Plugin struct {
	Name   string            `hcl:"name,label"`
	Driver string            `hcl:"driver"`
	Args   []string          `hcl:"args,optional"`
	Config map[string]string `hcl:"config,optional"`
}

// Policy holds the configuration information specific to the policy manager
// and resulting policy parsing.
type Policy struct {

	// Dir is the directory which contains scaling policies to be loaded from
	// disk. This currently only supports cluster scaling policies.
	Dir string `hcl:"dir,optional"`

	// DefaultCooldown is the default cooldown parameter added to all policies
	// which do not explicitly configure the parameter.
	DefaultCooldown    time.Duration
	DefaultCooldownHCL string `hcl:"default_cooldown,optional"`

	// DefaultEvaluationInterval is the time duration interval used when
	// `evaluation_interval` is not defined in a policy.
	DefaultEvaluationInterval    time.Duration
	DefaultEvaluationIntervalHCL string `hcl:"default_evaluation_interval,optional" json:"-"`

	// Sources store configuration for policy sources.
	Sources []*PolicySource `hcl:"source,block"`
}

// PolicyEval holds the configuration related to the policy evaluation process.
type PolicyEval struct {
	// DeliveryLimit is the maxmimum number of times a policy evaluation can
	// be dequeued from the broker.
	DeliveryLimitPtr *int `hcl:"delivery_limit,optional"`
	DeliveryLimit    int

	// AckTimeout is the time limit that an eval must be ACK'd before being
	// considered NACK'd.
	AckTimeout    time.Duration
	AckTimeoutHCL string `hcl:"ack_timeout,optional" json:"-"`

	// Workers hold the number of workers to initialize for each queue.
	Workers map[string]int `hcl:"workers,optional"`
}

// PolicySource is an individual configured policy source.
type PolicySource struct {
	Name    string `hcl:"name,label"`
	Enabled *bool  `hcl:"enabled,optional"`
}

const (
	// defaultLogLevel is the default log level used for the Autoscaler agent.
	defaultLogLevel = "info"

	// defaultHTTPBindAddress is the default address used for the HTTP health
	// server.
	defaultHTTPBindAddress = "127.0.0.1"

	// defaultHTTPBindPort is the default port used for the HTTP health server.
	defaultHTTPBindPort = 8080

	// defaultEvaluationInterval is the default value for the interval between evaluations
	defaultEvaluationInterval = time.Second * 10

	// defaultPluginDirSuffix is the suffix appended to the PWD when building
	// the PluginDir default value.
	defaultPluginDirSuffix = "/plugins"

	// defaultPolicyCooldown is the default time duration applied to policies
	// which do not explicitly configure a cooldown.
	defaultPolicyCooldown = 5 * time.Minute

	// defaultTelemetryCollectionInterval is the default telemetry metrics
	// collection interval.
	defaultTelemetryCollectionInterval = 1 * time.Second

	// defaultPolicyWorkerDeliveryLimit is the default value for the delivery
	// limit count for the policy eval broker.
	defaultPolicyEvalDeliveryLimit = 1

	// defaultPolicyWorkerAckTimeout is the default time limit that a policy
	// eval must be ACK'd.
	defaultPolicyEvalAckTimeout = 5 * time.Minute

	// defaultLockPath is the default path used for the lock that syncs the leader
	// election.
	defaultLockPath = "nomad-autoscaler/lock"
)

// TODO: there's an unexpected import cycle that prevents us from using the
// values defined in policy/source.go.
const (
	// policySourceFile is the source for policies that are loaded from disk.
	policySourceFile = "file"

	// policySourceNomad is the source for policies that originate from the
	// Nomad scaling policies API.
	policySourceNomad = "nomad"
)

var defaultPolicyEvalWorkers = map[string]int{
	"cluster":    10,
	"horizontal": 10,
}

// Default is used to generate a new default agent configuration.
func Default() (*Agent, error) {

	// Get the current working directory, so we can create the default
	// plugin_dir path.
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &Agent{
		LogLevel:                 defaultLogLevel,
		PluginDir:                pwd + defaultPluginDirSuffix,
		DynamicApplicationSizing: &DynamicApplicationSizing{},
		HTTP: &HTTP{
			BindAddress: defaultHTTPBindAddress,
			BindPort:    defaultHTTPBindPort,
		},
		Nomad: &Nomad{},
		Telemetry: &Telemetry{
			CollectionInterval: defaultTelemetryCollectionInterval,
		},
		Policy: &Policy{
			DefaultCooldown:           defaultPolicyCooldown,
			DefaultEvaluationInterval: defaultEvaluationInterval,
			Sources: []*PolicySource{
				{Name: policySourceFile, Enabled: ptr.BoolToPtr(true)},
				{Name: policySourceNomad, Enabled: ptr.BoolToPtr(true)},
			},
		},
		PolicyEval: &PolicyEval{
			DeliveryLimit: defaultPolicyEvalDeliveryLimit,
			AckTimeout:    defaultPolicyEvalAckTimeout,
			Workers:       defaultPolicyEvalWorkers,
		},
		APMs: []*Plugin{
			{Name: plugins.InternalAPMNomad, Driver: plugins.InternalAPMNomad},
		},
		Strategies: []*Plugin{
			{Name: plugins.InternalStrategyFixedValue, Driver: plugins.InternalStrategyFixedValue},
			{Name: plugins.InternalStrategyPassThrough, Driver: plugins.InternalStrategyPassThrough},
			{Name: plugins.InternalStrategyTargetValue, Driver: plugins.InternalStrategyTargetValue},
			{Name: plugins.InternalStrategyThreshold, Driver: plugins.InternalStrategyThreshold},
		},
		Targets: []*Plugin{
			{Name: plugins.InternalTargetNomad, Driver: plugins.InternalTargetNomad},
		},
		HighAvailability: &HighAvailability{
			Enable:   false,
			LockPath: defaultLockPath,
		},
	}, nil
}

// Merge is used to merge two agent configurations.
func (a *Agent) Merge(b *Agent) *Agent {
	if a == nil {
		return b
	}

	result := *a

	if b.EnableDebug {
		result.EnableDebug = true
	}
	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.LogJson {
		result.LogJson = true
	}
	if b.PluginDir != "" {
		result.PluginDir = b.PluginDir
	}

	if b.DynamicApplicationSizing != nil {
		result.DynamicApplicationSizing = result.DynamicApplicationSizing.merge(b.DynamicApplicationSizing)
	}

	if b.HighAvailability != nil {
		result.HighAvailability = result.HighAvailability.merge(b.HighAvailability)
	}

	if b.HTTP != nil {
		result.HTTP = result.HTTP.merge(b.HTTP)
	}

	if b.Nomad != nil {
		result.Nomad = result.Nomad.merge(b.Nomad)
	}

	if b.Telemetry != nil {
		result.Telemetry = result.Telemetry.merge(b.Telemetry)
	}

	if b.Policy != nil {
		result.Policy = result.Policy.merge(b.Policy)
	}

	if b.PolicyEval != nil {
		result.PolicyEval = result.PolicyEval.merge(b.PolicyEval)
	}

	if len(result.APMs) == 0 && len(b.APMs) != 0 {
		apmCopy := make([]*Plugin, len(b.APMs))
		for i, v := range b.APMs {
			apmCopy[i] = v.copy()
		}
		result.APMs = apmCopy
	} else if len(b.APMs) != 0 {
		result.APMs = pluginConfigSetMerge(result.APMs, b.APMs)
	}

	if len(result.Targets) == 0 && len(b.Targets) != 0 {
		targetCopy := make([]*Plugin, len(b.Targets))
		for i, v := range b.Targets {
			targetCopy[i] = v.copy()
		}
		result.Targets = targetCopy
	} else if len(b.Targets) != 0 {
		result.Targets = pluginConfigSetMerge(result.Targets, b.Targets)
	}

	if len(result.Strategies) == 0 && len(b.Strategies) != 0 {
		strategyCopy := make([]*Plugin, len(b.Strategies))
		for i, v := range b.Strategies {
			strategyCopy[i] = v.copy()
		}
		result.Strategies = strategyCopy
	} else if len(b.Strategies) != 0 {
		result.Strategies = pluginConfigSetMerge(result.Strategies, b.Strategies)
	}

	return &result
}

func (a *Agent) Validate() error {
	var result *multierror.Error

	modeChecker := NewModeChecker()
	result = multierror.Append(result, modeChecker.ValidateStruct(a))

	if a.PolicyEval != nil {
		result = multierror.Append(result, a.PolicyEval.validate())
	}

	if a.Policy != nil {
		for _, s := range a.Policy.Sources {
			result = multierror.Append(result, s.validate())
		}
	}

	return result.ErrorOrNil()
}

func (d *DynamicApplicationSizing) merge(b *DynamicApplicationSizing) *DynamicApplicationSizing {
	if d == nil {
		return b
	}

	result := *d

	if b.MetricsPreloadThreshold != 0 {
		result.MetricsPreloadThreshold = b.MetricsPreloadThreshold
	}
	if b.EvaluateAfter != 0 {
		result.EvaluateAfter = b.EvaluateAfter
	}
	if b.NamespaceLabel != "" {
		result.NamespaceLabel = b.NamespaceLabel
	}
	if b.JobLabel != "" {
		result.JobLabel = b.JobLabel
	}
	if b.GroupLabel != "" {
		result.GroupLabel = b.GroupLabel
	}
	if b.TaskLabel != "" {
		result.TaskLabel = b.TaskLabel
	}
	if b.CPUMetric != "" {
		result.CPUMetric = b.CPUMetric
	}
	if b.MemoryMetric != "" {
		result.MemoryMetric = b.MemoryMetric
	}

	return &result
}

func (h *HTTP) merge(b *HTTP) *HTTP {
	if h == nil {
		return b
	}

	result := *h

	if b.BindAddress != "" {
		result.BindAddress = b.BindAddress
	}
	if b.BindPort != 0 {
		result.BindPort = b.BindPort
	}

	return &result
}

func (n *Nomad) merge(b *Nomad) *Nomad {
	if n == nil {
		return b
	}

	result := *n

	if b.Address != "" {
		result.Address = b.Address
	}
	if b.Region != "" {
		result.Region = b.Region
	}
	if b.Namespace != "" {
		result.Namespace = b.Namespace
	}
	if b.Token != "" {
		result.Token = b.Token
	}
	if b.HTTPAuth != "" {
		result.HTTPAuth = b.HTTPAuth
	}
	if b.CACert != "" {
		result.CACert = b.CACert
	}
	if b.CAPath != "" {
		result.CAPath = b.CAPath
	}
	if b.ClientCert != "" {
		result.ClientCert = b.ClientCert
	}
	if b.ClientKey != "" {
		result.ClientKey = b.ClientKey
	}
	if b.TLSServerName != "" {
		result.TLSServerName = b.TLSServerName
	}
	if b.SkipVerify {
		result.SkipVerify = b.SkipVerify
	}

	return &result
}

func (t *Telemetry) merge(b *Telemetry) *Telemetry {
	if t == nil {
		return b
	}

	result := *t

	if b.StatsiteAddr != "" {
		result.StatsiteAddr = b.StatsiteAddr
	}
	if b.StatsdAddr != "" {
		result.StatsdAddr = b.StatsdAddr
	}
	if b.DogStatsDAddr != "" {
		result.DogStatsDAddr = b.DogStatsDAddr
	}
	if b.DogStatsDTags != nil {
		result.DogStatsDTags = b.DogStatsDTags
	}
	if b.PrometheusMetrics {
		result.PrometheusMetrics = b.PrometheusMetrics
	}
	if b.PrometheusRetentionTime != 0 {
		result.PrometheusRetentionTime = b.PrometheusRetentionTime
	}
	if b.DisableHostname {
		result.DisableHostname = true
	}
	if b.CollectionInterval != 0 {
		result.CollectionInterval = b.CollectionInterval
	}
	if b.CirconusAPIToken != "" {
		result.CirconusAPIToken = b.CirconusAPIToken
	}
	if b.CirconusAPIApp != "" {
		result.CirconusAPIApp = b.CirconusAPIApp
	}
	if b.CirconusAPIURL != "" {
		result.CirconusAPIURL = b.CirconusAPIURL
	}
	if b.CirconusCheckSubmissionURL != "" {
		result.CirconusCheckSubmissionURL = b.CirconusCheckSubmissionURL
	}
	if b.CirconusSubmissionInterval != "" {
		result.CirconusSubmissionInterval = b.CirconusSubmissionInterval
	}
	if b.CirconusCheckID != "" {
		result.CirconusCheckID = b.CirconusCheckID
	}
	if b.CirconusCheckForceMetricActivation != "" {
		result.CirconusCheckForceMetricActivation = b.CirconusCheckForceMetricActivation
	}
	if b.CirconusCheckInstanceID != "" {
		result.CirconusCheckInstanceID = b.CirconusCheckInstanceID
	}
	if b.CirconusCheckSearchTag != "" {
		result.CirconusCheckSearchTag = b.CirconusCheckSearchTag
	}
	if b.CirconusCheckTags != "" {
		result.CirconusCheckTags = b.CirconusCheckTags
	}
	if b.CirconusCheckDisplayName != "" {
		result.CirconusCheckDisplayName = b.CirconusCheckDisplayName
	}
	if b.CirconusBrokerID != "" {
		result.CirconusBrokerID = b.CirconusBrokerID
	}
	if b.CirconusBrokerSelectTag != "" {
		result.CirconusBrokerSelectTag = b.CirconusBrokerSelectTag
	}

	return &result
}

func (ha *HighAvailability) merge(b *HighAvailability) *HighAvailability {
	if ha == nil {
		return b
	}

	result := *ha
	if b.Enable {
		result.Enable = b.Enable
	}

	if b.LockPath != "" {
		result.LockPath = b.LockPath
	}

	return &result
}

func (p *Plugin) merge(o *Plugin) *Plugin {
	if p == nil {
		return o
	}

	m := *p

	if len(o.Name) != 0 {
		m.Name = o.Name
	}
	if len(o.Args) != 0 {
		m.Args = o.Args
	}
	if len(o.Config) != 0 {
		m.Config = o.Config
	}

	return m.copy()
}

func (p *Plugin) copy() *Plugin {
	if p == nil {
		return nil
	}

	c := *p
	if i, err := copystructure.Copy(p.Config); err != nil {
		panic(err.Error())
	} else {
		c.Config = i.(map[string]string)
	}
	return &c
}

func (p *Policy) merge(b *Policy) *Policy {
	if p == nil {
		return b
	}

	result := *p

	if b.Dir != "" {
		result.Dir = b.Dir
	}
	if b.DefaultCooldown != 0 {
		result.DefaultCooldown = b.DefaultCooldown
	}
	if b.DefaultEvaluationInterval != 0 {
		result.DefaultEvaluationInterval = b.DefaultEvaluationInterval
	}

	if len(result.Sources) == 0 && len(b.Sources) != 0 {
		sourceCopy := make([]*PolicySource, len(b.Sources))
		for i, v := range b.Sources {
			sourceCopy[i] = v.copy()
		}
		result.Sources = sourceCopy
	} else if len(b.Sources) != 0 {
		result.Sources = policySourceConfigSetMerge(result.Sources, b.Sources)
	}

	return &result
}

func (pw *PolicyEval) merge(in *PolicyEval) *PolicyEval {
	if pw == nil {
		return in
	}

	result := *pw

	if in.AckTimeout != 0 {
		result.AckTimeout = in.AckTimeout
	}

	if in.DeliveryLimitPtr != nil {
		result.DeliveryLimitPtr = in.DeliveryLimitPtr
		result.DeliveryLimit = in.DeliveryLimit
	}

	for k, v := range in.Workers {
		result.Workers[k] = v
	}

	return &result
}

func (pw *PolicyEval) validate() *multierror.Error {
	var result *multierror.Error
	prefix := "policy_workers ->"

	if pw.DeliveryLimitPtr != nil && pw.DeliveryLimit <= 0 {
		result = multierror.Append(result, errors.New("delivery_limit must be bigger than 0"))
	}

	for k, v := range pw.Workers {
		if v < 0 {
			result = multierror.Append(result, fmt.Errorf("number of workers for %q must be positive", k))
		}
	}

	// Prefix all errors.
	if result != nil {
		for i, err := range result.Errors {
			result.Errors[i] = multierror.Prefix(err, prefix)
		}
	}
	return result
}
func (s *PolicySource) copy() *PolicySource {
	if s == nil {
		return nil
	}

	var enabled *bool
	if s.Enabled != nil {
		enabled = ptr.BoolToPtr(*s.Enabled)
	}

	return &PolicySource{
		Name:    s.Name,
		Enabled: enabled,
	}
}

func (s *PolicySource) merge(b *PolicySource) *PolicySource {
	if s == nil {
		return b
	}

	result := *s

	if len(b.Name) != 0 {
		result.Name = b.Name
	}
	if b.Enabled != nil {
		result.Enabled = b.Enabled
	}

	return &result
}

func (s *PolicySource) validate() *multierror.Error {
	var result *multierror.Error
	prefix := fmt.Sprintf("source[%s] ->", s.Name)

	validSources := map[string]bool{
		policySourceNomad: true,
		policySourceFile:  true,
	}
	if _, ok := validSources[s.Name]; !ok {
		result = multierror.Append(result, fmt.Errorf("invalid source %q", s.Name))
	}

	// Prefix all errors.
	if result != nil {
		for i, err := range result.Errors {
			result.Errors[i] = multierror.Prefix(err, prefix)
		}
	}
	return result
}

// pluginConfigSetMerge merges two sets of plugin configs. For plugins with the
// same name, the configs are merged.
func pluginConfigSetMerge(first, second []*Plugin) []*Plugin {
	findex := make(map[string]*Plugin, len(first))
	for _, p := range first {
		findex[p.Name] = p
	}

	sindex := make(map[string]*Plugin, len(second))
	for _, p := range second {
		sindex[p.Name] = p
	}

	var out []*Plugin

	// Go through the first set and merge any value that exist in both
	for pluginName, original := range findex {
		second, ok := sindex[pluginName]
		if !ok {
			out = append(out, original.copy())
			continue
		}

		out = append(out, original.merge(second))
	}

	// Go through the second set and add any value that didn't exist in both
	for pluginName, plugin := range sindex {
		_, ok := findex[pluginName]
		if ok {
			continue
		}

		out = append(out, plugin)
	}

	return out
}

func policySourceConfigSetMerge(first, second []*PolicySource) []*PolicySource {
	findex := make(map[string]*PolicySource, len(first))
	for _, p := range first {
		findex[p.Name] = p
	}

	sindex := make(map[string]*PolicySource, len(second))
	for _, p := range second {
		sindex[p.Name] = p
	}

	var out []*PolicySource

	// Go through the first set and merge any value that exist in both
	for sourceName, original := range findex {
		second, ok := sindex[sourceName]
		if !ok {
			out = append(out, original.copy())
			continue
		}

		out = append(out, original.merge(second))
	}

	// Go through the second set and add any value that didn't exist in both
	for sourceName, source := range sindex {
		_, ok := findex[sourceName]
		if ok {
			continue
		}

		out = append(out, source)
	}

	return out
}

func parseFile(file string, cfg *Agent) error {
	if err := hclsimple.DecodeFile(file, nil, cfg); err != nil {
		return err
	}

	if cfg.Policy != nil {
		if cfg.Policy.DefaultCooldownHCL != "" {
			d, err := time.ParseDuration(cfg.Policy.DefaultCooldownHCL)
			if err != nil {
				return err
			}
			cfg.Policy.DefaultCooldown = d
		}

		if cfg.Policy.DefaultEvaluationIntervalHCL != "" {
			d, err := time.ParseDuration(cfg.Policy.DefaultEvaluationIntervalHCL)
			if err != nil {
				return err
			}
			cfg.Policy.DefaultEvaluationInterval = d
		}

		for _, source := range cfg.Policy.Sources {
			if source.Enabled == nil {
				// Default to true if source block is defined.
				source.Enabled = ptr.BoolToPtr(true)
			}
		}
	}

	if cfg.Telemetry != nil {
		if cfg.Telemetry.CollectionIntervalHCL != "" {
			d, err := time.ParseDuration(cfg.Telemetry.CollectionIntervalHCL)
			if err != nil {
				return err
			}
			cfg.Telemetry.CollectionInterval = d
		}
		if cfg.Telemetry.PrometheusRetentionTimeHCL != "" {
			d, err := time.ParseDuration(cfg.Telemetry.PrometheusRetentionTimeHCL)
			if err != nil {
				return err
			}
			cfg.Telemetry.PrometheusRetentionTime = d
		}
	}

	if cfg.PolicyEval != nil {
		if cfg.PolicyEval.AckTimeoutHCL != "" {
			t, err := time.ParseDuration(cfg.PolicyEval.AckTimeoutHCL)
			if err != nil {
				return err
			}
			cfg.PolicyEval.AckTimeout = t
		}

		if cfg.PolicyEval.DeliveryLimitPtr != nil {
			cfg.PolicyEval.DeliveryLimit = *cfg.PolicyEval.DeliveryLimitPtr
		}
	}

	if cfg.DynamicApplicationSizing != nil {
		if cfg.DynamicApplicationSizing.MetricsPreloadThresholdHCL != "" {
			t, err := time.ParseDuration(cfg.DynamicApplicationSizing.MetricsPreloadThresholdHCL)
			if err != nil {
				return err
			}
			cfg.DynamicApplicationSizing.MetricsPreloadThreshold = t
		}

		if cfg.DynamicApplicationSizing.EvaluateAfterHCL != "" {
			t, err := time.ParseDuration(cfg.DynamicApplicationSizing.EvaluateAfterHCL)
			if err != nil {
				return err
			}
			cfg.DynamicApplicationSizing.EvaluateAfter = t
		}
	}

	return nil
}

func LoadPaths(paths []string) (*Agent, error) {
	// Grab a default config as the base.
	cfg, err := Default()
	if err != nil {
		return nil, err
	}

	var validationErr *multierror.Error

	// Merge in the enterprise overlay.
	cfg = cfg.Merge(DefaultEntConfig())

	for _, path := range paths {
		current, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("error loading configuration from %s: %s", path, err)
		}

		if err := current.Validate(); err != nil {
			errPrefix := fmt.Sprintf("%s:", path)
			validationErr = multierror.Append(validationErr, multierror.Prefix(err, errPrefix))

			// Continue looping so we can validate other files.
			continue
		}

		if cfg == nil {
			cfg = current
		} else {
			cfg = cfg.Merge(current)
		}
	}

	if validationErr != nil {
		return nil, fmt.Errorf("invalid configuration. %v", validationErr)
	}

	return cfg, nil
}

// Load loads the configuration at the given path, regardless if its a file or
// directory. Called for each -config to build up the runtime config value.
func Load(path string) (*Agent, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return loadDir(path)
	}

	cleaned := filepath.Clean(path)

	cfg := &Agent{}
	if err := parseFile(cleaned, cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %v", cleaned, err)
	}
	return cfg, nil
}

// loadDir loads all the configurations in the given directory in alphabetical
// order.
func loadDir(dir string) (*Agent, error) {

	files, err := file.GetFileListFromDir(dir, ".hcl", ".json")
	if err != nil {
		return nil, fmt.Errorf("failed to load config directory: %v", err)
	}

	// Fast-path if we have no files
	if len(files) == 0 {
		return &Agent{}, nil
	}

	sort.Strings(files)

	var result *Agent
	for _, f := range files {

		cfg := &Agent{}

		if err := parseFile(f, cfg); err != nil {
			return nil, fmt.Errorf("error parsing config file %s: %v", f, err)
		}

		if result == nil {
			result = cfg
		} else {
			result = result.Merge(cfg)
		}
	}

	return result, nil
}
