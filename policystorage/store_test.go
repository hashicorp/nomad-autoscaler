package policystorage

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func Test_NewStore(t *testing.T) {
	ns := NewStore(hclog.Default(), nil)
	assert.NotNil(t, ns)
	assert.Nil(t, ns.nomad)
	assert.Equal(t, "policy-storage", ns.log.Name())
	assert.Len(t, ns.State.List(), 0)
}
