package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/nomad-autoscaler/helper/file"
	"github.com/hashicorp/nomad-autoscaler/plugins"
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

	// PluginDir is the directory that holds the autoscaler plugin binaries.
	PluginDir string `hcl:"plugin_dir,optional"`

	// HTTP is the configuration used to setup the HTTP health server.
	HTTP *HTTP `hcl:"http,block"`

	// Nomad is the configuration used to setup the Nomad client.
	Nomad *Nomad `hcl:"nomad,block"`

	// Policy is the configuration used to setup the policy manager.
	Policy *Policy `hcl:"policy,block"`

	APMs       []*Plugin `hcl:"apm,block"`
	Targets    []*Plugin `hcl:"target,block"`
	Strategies []*Plugin `hcl:"strategy,block"`
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
}

const (
	// defaultLogLevel is the default log level used for the Autoscaler agent.
	defaultLogLevel = "info"

	// defaultHTTPBindAddress is the default address used for the HTTP health
	// server.
	defaultHTTPBindAddress = "127.0.0.1"

	// defaultHTTPBindPort is the default port used for the HTTP health server.
	defaultHTTPBindPort = 8080

	// defaultEvaluationInterval is the default value for the
	// DefaultEvaluationInterval in nano seconds.
	defaultEvaluationInterval = time.Duration(10000000000)

	// defaultPluginDirSuffix is the suffix appended to the PWD when building
	// the PluginDir default value.
	defaultPluginDirSuffix = "/plugins"

	// defaultNomadAddress is the default address used for Nomad API
	// connectivity.
	defaultNomadAddress = "http://127.0.0.1:4646"

	// defaultNomadRegion is the default Nomad region to use when performing
	// Nomad API calls.
	defaultNomadRegion = "global"

	// defaultPolicyCooldown is the default time duration applied to policies
	// which do not explicitly configure a cooldown.
	defaultPolicyCooldown = 5 * time.Minute
)

// Default is used to generate a new default agent configuration.
func Default() (*Agent, error) {

	// Get the current working directory, so we can create the default
	// plugin_dir path.
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &Agent{
		LogLevel:  defaultLogLevel,
		PluginDir: pwd + defaultPluginDirSuffix,
		HTTP: &HTTP{
			BindAddress: defaultHTTPBindAddress,
			BindPort:    defaultHTTPBindPort,
		},
		Nomad: &Nomad{
			Address: defaultNomadAddress,
			Region:  defaultNomadRegion,
		},
		Policy: &Policy{
			DefaultCooldown:           defaultPolicyCooldown,
			DefaultEvaluationInterval: defaultEvaluationInterval,
		},
		APMs:       []*Plugin{{Name: plugins.InternalAPMNomad, Driver: plugins.InternalAPMNomad}},
		Strategies: []*Plugin{{Name: plugins.InternalStrategyTargetValue, Driver: plugins.InternalStrategyTargetValue}},
		Targets:    []*Plugin{{Name: plugins.InternalTargetNomad, Driver: plugins.InternalTargetNomad}},
	}, nil
}

// Merge is used to merge two agent configurations.
func (a *Agent) Merge(b *Agent) *Agent {
	result := *a

	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.LogJson {
		result.LogJson = true
	}
	if b.PluginDir != "" {
		result.PluginDir = b.PluginDir
	}
	if b.HTTP != nil {
		result.HTTP = result.HTTP.merge(b.HTTP)
	}

	if b.Nomad != nil {
		result.Nomad = result.Nomad.merge(b.Nomad)
	}

	if b.Policy != nil {
		result.Policy = result.Policy.merge(b.Policy)
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

func (h *HTTP) merge(b *HTTP) *HTTP {
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

func (p *Plugin) merge(o *Plugin) *Plugin {
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
	c := *p
	if i, err := copystructure.Copy(p.Config); err != nil {
		panic(err.Error())
	} else {
		c.Config = i.(map[string]string)
	}
	return &c
}

func (p *Policy) merge(b *Policy) *Policy {
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
	return &result
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

func parseFile(file string, cfg *Agent) error {
	if err := hclsimple.DecodeFile(file, nil, cfg); err != nil {
		return err
	}

	if cfg.Policy == nil {
		return nil
	}

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

	return nil
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
