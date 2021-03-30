package nodeselector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_emptyClusterScaleInNodeSelectorName(t *testing.T) {
	assert.Equal(t, "empty", newEmptyClusterScaleInNodeSelector(nil, nil).Name())
}
