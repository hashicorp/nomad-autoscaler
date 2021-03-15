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

func TestFuncMapStringIntVar(t *testing.T) {
	var result map[string]int

	sv := FuncMapStringIngVar(func(m map[string]int) error {
		result = m
		return nil
	})
	err := sv.Set("a:1,b:2")

	assert.NoError(t, err)
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, result)
	assert.Equal(t, "", sv.String())

	err = sv.Set("a:invalid")
	assert.Error(t, err)
}
