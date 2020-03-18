package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
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

	// ScanInterval is the time duration interval at which the autoscaler runs
	// evaluations.
	ScanInterval    time.Duration
	ScanIntervalHCL string `hcl:"scan_interval,optional" json:"-"`

	// Nomad is the configuration used to setup the Nomad client.
	Nomad *Nomad `hcl:"nomad,block"`

	APMs       []*Plugin `hcl:"apm,block"`
	Targets    []*Plugin `hcl:"target,block"`
	Strategies []*Plugin `hcl:"strategy,block"`
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

const (
	// defaultLogLevel is the default log level used for the Autoscaler agent.
	defaultLogLevel = "info"

	// defaultScanInterval is the default value for the ScaInterval in nano
	// seconds.
	defaultScanInterval = time.Duration(10000000000)

	// defaultPluginDirSuffix is the suffix appended to the PWD when building
	// the PluginDir default value.
	defaultPluginDirSuffix = "/plugins"

	// defaultNomadAddress is the default address used for Nomad API
	// connectivity.
	defaultNomadAddress = "http://127.0.0.1:4646"

	// defaultNomadRegion is the default Nomad region to use when performing
	// Nomad API calls.
	defaultNomadRegion = "global"
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
		LogLevel:     defaultLogLevel,
		PluginDir:    pwd + defaultPluginDirSuffix,
		ScanInterval: defaultScanInterval,
		Nomad: &Nomad{
			Address: defaultNomadAddress,
			Region:  defaultNomadRegion,
		},
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
	if b.ScanInterval != 0 {
		result.ScanInterval = b.ScanInterval
	}

	if b.Nomad != nil {
		result.Nomad = result.Nomad.merge(b.Nomad)
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

	if cfg.ScanIntervalHCL != "" {
		d, err := time.ParseDuration(cfg.ScanIntervalHCL)
		if err != nil {
			return err
		}
		cfg.ScanInterval = d
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
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("configuration path must be a directory: %s", dir)
	}

	var files []string
	err = nil
	for err != io.EOF {
		var fis []os.FileInfo
		fis, err = f.Readdir(128)
		if err != nil && err != io.EOF {
			return nil, err
		}

		for _, fi := range fis {
			// Ignore directories
			if fi.IsDir() {
				continue
			}

			// Only care about files that are valid to load.
			name := fi.Name()
			skip := true
			if strings.HasSuffix(name, ".hcl") {
				skip = false
			} else if strings.HasSuffix(name, ".json") {
				skip = false
			}
			if skip || isTemporaryFile(name) {
				continue
			}

			path := filepath.Join(dir, name)
			files = append(files, path)
		}
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

// isTemporaryFile returns true or false depending on whether the provided file
// name is a temporary file for the following editors: emacs or vim.
func isTemporaryFile(name string) bool {
	return strings.HasSuffix(name, "~") || // vim
		strings.HasPrefix(name, ".#") || // emacs
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#")) // emacs
}
