package targetstrategy

import (
	"log"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/strategy"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

type TargetStrategy struct {
	Config map[string]string
}

func (apms *TargetStrategy) Run(config map[string]interface{}) ([]strategy.StrategyResult, error) {
	log.Println("ran strategy")
	return []strategy.StrategyResult{{}}, nil
}

func main() {
	s := &TargetStrategy{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"strategy": &strategy.Plugin{Impl: s},
		},
	})

}
