// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policyeval

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBroker(t *testing.T) {
	assert := assert.New(t)

	l := hclog.Default()
	l.SetLevel(hclog.Debug)

	nackTimeout := 100 * time.Millisecond

	// Setup broker so it only allows dequeueing evals twice before failing.
	b := NewBroker(l, nackTimeout, 2)

	// Create and enqueue some evals.
	eval1 := &sdk.ScalingEvaluation{
		ID: "eval1",
		Policy: &sdk.ScalingPolicy{
			ID:   "policy1",
			Type: "horizontal",
		},
		CreateTime: time.Date(2020, time.October, 12, 23, 0, 0, 0, time.UTC),
	}
	eval1b := &sdk.ScalingEvaluation{
		ID:         "eval1b",
		Policy:     eval1.Policy,
		CreateTime: time.Date(2020, time.October, 12, 22, 0, 0, 0, time.UTC),
	}
	eval1c := &sdk.ScalingEvaluation{
		ID:         "eval1c",
		Policy:     eval1.Policy,
		CreateTime: time.Date(2020, time.October, 12, 21, 0, 0, 0, time.UTC),
	}

	// Create an older eval.
	eval2 := &sdk.ScalingEvaluation{
		ID: "eval2",
		Policy: &sdk.ScalingPolicy{
			ID:   "policy2",
			Type: "horizontal",
		},
		CreateTime: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}

	// Crete high priority policy.
	eval3 := &sdk.ScalingEvaluation{
		ID: "eval3",
		Policy: &sdk.ScalingPolicy{
			ID:       "policy3",
			Type:     "horizontal",
			Priority: 10,
		},
	}

	// Enqueue the first eval for policy1.
	b.Enqueue(eval1b)
	assert.Equal(eval1b, b.pendingEvals["horizontal"][0])

	// Enqueue eval1 and see if replaced eval1b since it has the same policy
	// but was created before.
	b.Enqueue(eval1)
	assert.Equal(eval1, b.pendingEvals["horizontal"][0])

	// Try to enqueue another eval for policy1 that is older than the current
	// enqueued eval.
	b.Enqueue(eval1c)
	assert.Equal(eval1, b.pendingEvals["horizontal"][0])

	// Make sure only one eval was enqueued.
	assert.Equal(b.pendingEvals["horizontal"].Len(), 1)

	// Enqueue other evals.
	b.Enqueue(eval2)
	b.Enqueue(eval3)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Don't ack eval before the nack timer is triggered.
	time.Sleep(2 * nackTimeout)
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
	assert.NoError(ctx.Err())
	assert.Empty(token)
	assert.Nil(err)
}
