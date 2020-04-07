package policy

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestNewWatcher(t *testing.T) {
	w := NewWatcher(hclog.Default(), nil, nil)
	assert.Equal(t, "policy_watcher", w.log.Name())
}
