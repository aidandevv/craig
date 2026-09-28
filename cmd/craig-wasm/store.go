//go:build js && wasm

package main

import (
	"context"
	"errors"
	"syscall/js"
	"time"

	"github.com/aidandevv/craig-extension/internal/browserapi"
)

// hostStore implements signals.EvidenceStore over the extension's craigStore.
type hostStore struct{ host js.Value }

func (s hostStore) GetEvidence(_ context.Context, hash, feature string) (map[string]any, time.Time, bool) {
	value, err := await(s.host.Call("getEvidence", hash, feature))
	if err != nil || value.Type() != js.TypeString {
		return nil, time.Time{}, false
	}
	return browserapi.DecodeEvidence(value.String(), time.Now())
}

func (s hostStore) PutEvidence(_ context.Context, hash, feature string, data map[string]any, at time.Time) error {
	encoded, err := browserapi.EncodeEvidence(data)
	if err != nil {
		return err
	}
	_, err = await(s.host.Call("putEvidence", hash, feature, encoded, at.UTC().Format(time.RFC3339Nano)))
	return err
}

func (s hostStore) ReserveVisionUnit(_ context.Context, feature string, monthlyCap int) (bool, error) {
	value, err := await(s.host.Call("reserveUnit", feature, monthlyCap))
	if err != nil {
		return false, err
	}
	return value.Type() == js.TypeBoolean && value.Bool(), nil
}

// await blocks the calling goroutine until a JS Promise settles. It must never
// be called from the main goroutine or inside a js.FuncOf callback.
func await(promise js.Value) (js.Value, error) {
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
	promise.Call("then", onFulfilled, onRejected)
	result := <-done
	return result.value, result.err
}

func argOrUndefined(args []js.Value) js.Value {
	if len(args) == 0 {
		return js.Undefined()
	}
	return args[0]
}
