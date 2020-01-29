package agent

type Config struct {
	PluginDir    string   `hcl:"plugin_dir"`
	ScanInterval string   `hcl:"scan_interval"`
	Nomad        Nomad    `hcl:"nomad,block"`
	APMs         []APM    `hcl:"apm,block"`
	Targets      []Target `hcl:"target,block"`
	Strategies   []Target `hcl:"strategy,block"`
}

type Nomad struct {
	Address string `hcl:"address"`
	Region  string `hcl:"region,optional"`
}

type APM struct {
	Name   string            `hcl:"name,label"`
	Driver string            `hcl:"driver"`
	Args   []string          `hcl:"args,optional"`
	Config map[string]string `hcl:"config,optional"`
}

type Target struct {
	Name   string            `hcl:"name,label"`
	Driver string            `hcl:"driver"`
	Args   []string          `hcl:"args,optional"`
	Config map[string]string `hcl:"config,optional"`
}

type Strategy struct {
	Name   string            `hcl:"name,label"`
	Driver string            `hcl:"driver"`
	Args   []string          `hcl:"args,optional"`
	Config map[string]string `hcl:"config,optional"`
}
