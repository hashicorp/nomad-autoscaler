package ha

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GenerateAgentID(t *testing.T) {
	knownID := "45fa3139-7b99-d800-e579-388ad57c10ff"

	testCases := []struct {
		f    func() error
		name string
	}{
		{
			f: func() error {
				if err := os.Setenv("NOMAD_ALLOCATION_ID", knownID); err != nil {
					return err
				}

				id, err := GenerateAgentID()
				if err != nil {
					return err
				}

				if id != knownID {
					return fmt.Errorf("agentID %q not same as expected %q", id, knownID)
				}
				return os.Unsetenv("NOMAD_ALLOCATION_ID")
			},
			name: "nomad allocation env var",
		},
		{
			f: func() error {

				dir, err := os.Getwd()
				if err != nil {
					return err
				}

				if err := ioutil.WriteFile(filepath.Join(dir, "nomad-autoscaler-id"), []byte(knownID), 0666); err != nil {
					return err
				}

				id, err := GenerateAgentID()
				if err != nil {
					return err
				}

				if id != knownID {
					return fmt.Errorf("agentID %q not same as expected %q", id, knownID)
				}
				return os.Remove(filepath.Join(dir, "nomad-autoscaler-id"))
			},
			name: "id file present with entry",
		},
		{
			f: func() error {
				dir, err := os.Getwd()
				if err != nil {
					return err
				}

				id, err := GenerateAgentID()
				if err != nil {
					return err
				}

				if id == "" {
					return errors.New("no agent ID set")
				}

				if id == knownID {
					return fmt.Errorf("agentID %q should not be the known ID %q", id, knownID)
				}
				return os.Remove(filepath.Join(dir, "nomad-autoscaler-id"))
			},
			name: "env var and file not present",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Nil(t, tc.f(), tc.name)
		})
	}
}
