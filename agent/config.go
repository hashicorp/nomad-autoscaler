package agent

type Config struct {
	PluginDir    string `hcl:"plugin_dir"`
	ScanInterval string `hcl:"scan_interval"`
	APMs         []APM  `hcl:"apm,block"`
}

type APM struct {
	Name   string            `hcl:"name,label"`
	Driver string            `hcl:"driver"`
	Args   []string          `hcl:"args,optional"`
	Config map[string]string `hcl:"config"`
}
