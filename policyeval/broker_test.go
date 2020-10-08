package policyeval

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestBroker(t *testing.T) {
	assert := assert.New(t)

	l := hclog.Default()
	l.SetLevel(hclog.Debug)

	// Setup broker so it only allows dequeueing evals twice before failing.
	b := NewBroker(hclog.Default(), time.Second, 2)

	// Create and enqueue some evals.
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Check if broker dedups evals.
	b.Enqueue(eval2)
	assert.Equal(3, b.pendingEvals["horizontal"].Len())

	// Check if eval3 is first, since it has the highest priority.
	e, token, err := b.Dequeue(ctx, "horizontal")
	assert.NoError(err)
	assert.Equal(eval3, e)
	assert.NotEmpty(token)

	// Ack eval3.
	err = b.Ack(e.ID, token)
	assert.NoError(err)

	// Check if eval2 is next since it's older.
	e, token, err = b.Dequeue(ctx, "horizontal")
	assert.NoError(err)
	assert.Equal(eval2, e)
	assert.NotEmpty(token)

	// Nack eval2 and see if pops out again.
	err = b.Nack(e.ID, token)
	assert.NoError(err)
	e, token, err = b.Dequeue(ctx, "horizontal")
	assert.NoError(err)
	assert.Equal(eval2, e)
	assert.NotEmpty(token)

	// Nack eval2 again and it should be marked as failed since the broker is
	// configured to only dequeue twice.
	err = b.Nack(e.ID, token)
	assert.NoError(err)
	e, token, err = b.Dequeue(ctx, "horizontal")
	assert.NoError(err)
	assert.NotEqual(eval2, e)
	// It should be eval1
	assert.Equal(eval1, e)
	assert.NotEmpty(token)

	// Ack with wrong token, and it should fail.
	err = b.Ack(e.ID, "not-the-chosen-one")
	assert.Error(err)
	// Ack with the right token
	err = b.Ack(e.ID, token)
	assert.NoError(err)

	// Wait for work that will arrive after 1s.
	eval4 := &sdk.ScalingEvaluation{
		ID: uuid.Generate(),
		Policy: &sdk.ScalingPolicy{
			ID:   uuid.Generate(),
			Type: "horizontal",
		},
	}
	go func() {
		time.Sleep(time.Second)
		b.Enqueue(eval4)
	}()
	// Dequeue will block until eval4 is enqueued.
	e, token, err = b.Dequeue(ctx, "horizontal")
	assert.NoError(err)
	assert.Equal(eval4, e)
	assert.NotEmpty(token)
	// Ack eval.
	b.Ack(e.ID, token)

	// Wait for work, but timeout afer 1s.
	ctxTO, cancelTO := context.WithTimeout(context.Background(), time.Second)
	defer cancelTO()
	e, token, err = b.Dequeue(ctxTO, "horizontal")
	<-ctxTO.Done()
	assert.Nil(e)
}
