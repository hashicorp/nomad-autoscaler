package status

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestNewWatcher(t *testing.T) {
	w := NewWatcher(hclog.Default(), "example", nil, nil)
	assert.Equal(t, "status-watcher-example", w.log.Name())
}
