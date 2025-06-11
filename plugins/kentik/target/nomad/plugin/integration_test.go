//go:build integration_test

package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_StatusIntegration(t *testing.T) {
	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": "https://nomad.our1.kentik.com",
	})

	for {
		got, err := plugin.Status(map[string]string{
			"Job":                "cd-functest",
			"Group":              "cd-functest",
			"Namespace":          "default",
			"CheckUnknownAllocs": "true",
		})
		if err == nil {
			assert.Equal(t, 9, int(got.Count))
		} else {
			fmt.Printf("err %v", err)
		}
		time.Sleep(time.Second)
	}
}

func TestTargetPlugin_InilegibleIntegration(t *testing.T) {
	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": "https://nomad.iad1.kentik.com",
	})

	for {
		got, err := plugin.Status(map[string]string{
			"Job":                "cd-functest",
			"Group":              "cd-functest",
			"Namespace":          "default",
			"CheckUnknownAllocs": "true",
		})
		if err == nil {
			assert.Equal(t, 1, int(got.Count))
		} else {
			fmt.Printf("err %v", err)
		}
		time.Sleep(time.Second)
	}
}
