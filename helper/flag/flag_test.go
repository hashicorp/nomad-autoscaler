package flag

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringFlag(t *testing.T) {
	sv := new(StringFlag)
	assert.Nil(t, sv.Set("foo"))
	assert.Nil(t, sv.Set("bar"))
	assert.Equal(t, []string{"foo", "bar"}, []string(*sv))
	assert.Equal(t, "foo,bar", sv.String())
}

func TestFuncDurationVar(t *testing.T) {
	var dur time.Duration

	sv := FuncDurationVar(func(d time.Duration) error {
		dur = d
		return nil
	})

	assert.Nil(t, sv.Set("30s"))
	assert.Equal(t, time.Duration(30000000000), dur)
	assert.Equal(t, "", sv.String())
	assert.False(t, sv.IsBoolFlag())
}
