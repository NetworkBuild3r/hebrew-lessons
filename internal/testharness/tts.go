package testharness

import (
	"context"
	"sync"
)

// FakeTTS is an audio.TTS that returns deterministic bytes.
// The bytes are the prefix "fake:" plus the text. Voice and model do not
// change the bytes. Calls counts every Synthesize that gets past a
// canceled context, including calls that return an error.
type FakeTTS struct {
	mu       sync.Mutex
	calls    map[string]int
	fail     map[string]error
	failNext error
}

// NewFakeTTS returns a synthesizer that succeeds until FailText or FailNext.
func NewFakeTTS() *FakeTTS {
	return &FakeTTS{
		calls: map[string]int{},
		fail:  map[string]error{},
	}
}

// Synthesize returns deterministic bytes for textNFC, or a configured error.
func (f *FakeTTS) Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, _ = voice, model
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[textNFC]++
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		return nil, err
	}
	if err := f.fail[textNFC]; err != nil {
		return nil, err
	}
	return []byte("fake:" + textNFC), nil
}

// Calls is how many times Synthesize has run for textNFC.
func (f *FakeTTS) Calls(textNFC string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[textNFC]
}

// FailText makes later Synthesize calls for textNFC return err.
// A nil err clears a previous failure for that text.
func (f *FakeTTS) FailText(textNFC string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == nil {
		f.fail = map[string]error{}
	}
	if err == nil {
		delete(f.fail, textNFC)
		return
	}
	f.fail[textNFC] = err
}

// FailNext makes the next Synthesize call return err, once.
// A nil err clears a pending failure.
func (f *FakeTTS) FailNext(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext = err
}
