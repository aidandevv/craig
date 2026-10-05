//go:build js && wasm

package main

import (
	"context"
	"errors"
	"syscall/js"
	"time"

	"github.com/aidandevv/craig/internal/browserapi"
)

// hostStore implements signals.EvidenceStore over the extension's craigStore.
type hostStore struct{ host js.Value }

func (s hostStore) GetEvidence(ctx context.Context, hash, feature string) (map[string]any, time.Time, bool) {
	value, err := await(ctx, s.host.Call("getEvidence", hash, feature))
	if err != nil || value.Type() != js.TypeString {
		return nil, time.Time{}, false
	}
	return browserapi.DecodeEvidence(value.String(), time.Now())
}

func (s hostStore) PutEvidence(ctx context.Context, hash, feature string, data map[string]any, at time.Time) error {
	encoded, err := browserapi.EncodeEvidence(data)
	if err != nil {
		return err
	}
	_, err = await(ctx, s.host.Call("putEvidence", hash, feature, encoded, at.UTC().Format(time.RFC3339Nano)))
	return err
}

func (s hostStore) ReserveVisionUnit(ctx context.Context, feature string, monthlyCap int) (bool, error) {
	value, err := await(ctx, s.host.Call("reserveUnit", feature, monthlyCap))
	if err != nil {
		return false, err
	}
	return value.Type() == js.TypeBoolean && value.Bool(), nil
}

// await blocks the calling goroutine until a JS Promise settles. It must never
// be called from the main goroutine or inside a js.FuncOf callback.
func await(ctx context.Context, promise js.Value) (js.Value, error) {
	type settled struct {
		value js.Value
		err   error
	}
	done := make(chan settled, 1)
	onFulfilled := js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- settled{value: argOrUndefined(args)}
		return nil
	})
	onRejected := js.FuncOf(func(_ js.Value, args []js.Value) any {
		// String(reason) rather than reason.toString(): it never panics, even
		// for undefined, null, or a primitive rejection reason.
		reason := js.Global().Get("String").Invoke(argOrUndefined(args)).String()
		done <- settled{err: errors.New(reason)}
		return nil
	})
	defer onFulfilled.Release()
	defer onRejected.Release()
	// Race in JavaScript so late host settlement cannot invoke released Go callbacks.
	timeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < timeout {
		timeout = time.Until(deadline)
	}
	if timeout < 0 {
		timeout = 0
	}
	var timer js.Value
	var timeoutCallback js.Func
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		reject := args[1]
		timeoutCallback = js.FuncOf(func(js.Value, []js.Value) any {
			reject.Invoke(js.Global().Get("Error").New("storage operation timed out"))
			return nil
		})
		timer = js.Global().Call("setTimeout", timeoutCallback, timeout.Milliseconds())
		return nil
	})
	timed := js.Global().Get("Promise").New(executor)
	executor.Release()
	defer timeoutCallback.Release()
	defer js.Global().Call("clearTimeout", timer)
	promises := js.Global().Get("Array").New()
	promises.Call("push", promise, timed)
	js.Global().Get("Promise").Call("race", promises).Call("then", onFulfilled, onRejected)
	result := <-done
	return result.value, result.err
}

func argOrUndefined(args []js.Value) js.Value {
	if len(args) == 0 {
		return js.Undefined()
	}
	return args[0]
}
