package policystorage

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
)

type Store struct {
	log             hclog.Logger
	nomad           *api.Client
	lastChangeIndex uint64

	State State
}

// NewStore creates a new policy store for interaction and control over the
// autoscalers internal policy storage.
func NewStore(log hclog.Logger, nomad *api.Client) *Store {
	return &Store{
		log:   log.ResetNamed("policy-storage"),
		nomad: nomad,
		State: newStateBackend(),
	}
}
