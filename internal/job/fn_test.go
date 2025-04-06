package job

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFnControl_SaveData(t *testing.T) {
	ctrl := &FnControl{
		data: &sync.Map{},
	}

	ctrl.SaveData(map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	})

	val1, ok1 := ctrl.data.Load("key1")
	val2, ok2 := ctrl.data.Load("key2")

	assert.True(t, ok1)
	assert.Equal(t, "value1", val1)

	assert.True(t, ok2)
	assert.Equal(t, 123, val2)
}

func TestFnControl_Context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctrl := &FnControl{ctx: ctx}
	assert.Equal(t, ctx, ctrl.Context())
}

func TestFnControl_PauseChan(t *testing.T) {
	ch := make(chan struct{}, 1)
	ctrl := &FnControl{pauseChan: ch}

	go func() {
		ch <- struct{}{}
	}()

	select {
	case <-ctrl.PauseChan():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("PauseChan was not received")
	}
}

func TestFnControl_ResumeChan_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := &FnControl{
		ctx:        ctx,
		resumeChan: make(chan struct{}),
	}

	cancel()

	select {
	case _, ok := <-ctrl.ResumeChan():
		assert.False(t, ok, "Expected ResumeChan to be closed after context cancel")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ResumeChan did not close after context cancel")
	}
}

func TestFnControl_ResumeChan_ReceiveSignal(t *testing.T) {
	ctx := context.Background()
	ch := make(chan struct{}, 1)
	ctrl := &FnControl{
		ctx:        ctx,
		resumeChan: ch,
	}

	ch <- struct{}{}

	select {
	case <-ctrl.ResumeChan():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ResumeChan did not receive signal")
	}
}
