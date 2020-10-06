package policyeval

import (
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestBroker(t *testing.T) {
	assert := assert.New(t)

	b := NewBroker(hclog.Default(), time.Second, 1)

	eval1 := &sdk.ScalingEvaluation{
		ID: uuid.Generate(),
		Policy: &sdk.ScalingPolicy{
			ID:   uuid.Generate(),
			Type: "horizontal",
		},
		CreateTime: time.Date(2020, time.October, 12, 23, 0, 0, 0, time.UTC),
	}

	// Create an older eval.
	eval2 := &sdk.ScalingEvaluation{
		ID: uuid.Generate(),
		Policy: &sdk.ScalingPolicy{
			ID:   uuid.Generate(),
			Type: "horizontal",
		},
		CreateTime: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}

	// Crete high priority policy.
	eval3 := &sdk.ScalingEvaluation{
		ID: uuid.Generate(),
		Policy: &sdk.ScalingPolicy{
			ID:       uuid.Generate(),
			Type:     "horizontal",
			Priority: 10,
		},
	}

	b.Enqueue(eval1)
	b.Enqueue(eval2)
	b.Enqueue(eval3)

	// Check if eval3 is first, since it has the highest priority.
	e, token, err := b.Dequeue(nil, "horizontal")
	assert.NoError(err)
	assert.Equal(eval3, e)
	assert.NotEmpty(token)

	err = b.Ack(e.ID, token)
	assert.NoError(err)

	// Check if eval2 is next since it's older.
	e, token, err = b.Dequeue(nil, "horizontal")
	assert.NoError(err)
	assert.Equal(eval2, e)
	assert.NotEmpty(token)

	// Nack eval2 and see if pops out again.
	err = b.Nack(e.ID, token)
	assert.NoError(err)

	e, token, err = b.Dequeue(nil, "horizontal")
	assert.NoError(err)
	assert.Equal(eval2, e)
	assert.NotEmpty(token)

}
