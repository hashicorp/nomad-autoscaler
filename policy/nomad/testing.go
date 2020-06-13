package nomad

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
)

// TestNomadSource returns a default policy.Source that retrieves policies
// from Nomad.
//
// The Nomad client and the agent can be configured by passing a cb function.
func TestNomadSource(t *testing.T, cb func(*api.Config, *SourceConfig)) *Source {
	nomadConfig := api.DefaultConfig()
	sourceConfig := &SourceConfig{}

	if cb != nil {
		cb(nomadConfig, sourceConfig)
	}

	nomad, err := api.NewClient(nomadConfig)
	if err != nil {
		t.Fatal(err)
	}

	log := hclog.New(&hclog.LoggerOptions{
		Level: hclog.Trace,
	})
	return NewNomadSource(log, nomad, sourceConfig)
}

func testParseJob(t *testing.T, path string) *api.Job {
	jobJSON, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read job file %s: %v", path, err)
	}

	// Partially read the JSON to see if we have a "Job" root.
	var root map[string]json.RawMessage
	err = json.Unmarshal(jobJSON, &root)
	if err != nil {
		t.Fatalf("failed to read job file %s: %v", path, err)
	}

	jobBytes, ok := root["Job"]
	if !ok {
		// Parse the input as is if there's no "Job" root.
		jobBytes = jobJSON
	}

	// Parse job bytes.
	var job api.Job
	err = json.Unmarshal(jobBytes, &job)
	if err != nil {
		t.Fatalf("failed to unmarshal job %s: %v", path, err)
	}

	return &job
}
