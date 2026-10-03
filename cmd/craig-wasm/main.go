//go:build js && wasm

// Command craig-wasm exposes the Craig engine to the browser extension. It
// registers Promise-returning functions on globalThis, signals readiness, and
// then blocks; the extension's service worker owns its lifetime.
package main

import (
	"context"
	"syscall/js"
	"time"

	"github.com/aidandevv/craig-extension/internal/browserapi"
	"github.com/aidandevv/craig-extension/internal/signals"
)

func main() {
	var store signals.EvidenceStore
	if craigStore := js.Global().Get("craigStore"); craigStore.Type() == js.TypeObject {
		// Only wire up storage when the host actually provided an object;
		// otherwise leave Host.Store nil so Vision stays disabled instead of
		// panicking the first time hostStore calls a method on undefined.
		store = hostStore{craigStore}
	}
	host := browserapi.Host{Store: store}
	js.Global().Set("craigAnalyze", promiseFunc(func(in string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return host.Analyze(ctx, []byte(in))
	}))
	js.Global().Set("craigPrepareRules", promiseFunc(func(in string) ([]byte, error) {
		return browserapi.PrepareRules([]byte(in))
	}))
	js.Global().Set("craigDefaultRules", promiseFunc(func(string) ([]byte, error) { return browserapi.DefaultRules() }))
	js.Global().Set("craigRuleSchema", promiseFunc(func(string) ([]byte, error) { return browserapi.RuleSchema() }))
	if ready := js.Global().Get("craigEngineReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

var activeCalls = make(chan struct{}, 4)

// promiseFunc adapts a blocking handler to a JS function returning a Promise
// of a string. Each call runs on its own goroutine so it can await host
// storage and fetch without stalling the JS event loop.
func promiseFunc(handler func(string) ([]byte, error)) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		input := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			input = args[0].String()
		}
		executor := js.FuncOf(func(_ js.Value, p []js.Value) any {
			resolve, reject := p[0], p[1]
			if len(input) > browserapi.MaxRequestBytes {
				reject.Invoke(js.Global().Get("Error").New("request exceeds size limit"))
				return nil
			}
			select {
			case activeCalls <- struct{}{}:
			default:
				reject.Invoke(js.Global().Get("Error").New("engine is busy"))
				return nil
			}
			go func() {
				defer func() { <-activeCalls }()
				// A panic here (e.g. the host violating the craigStore
				// contract) must become a rejected promise, not a dead
				// wasm runtime that fails every later call.
				defer func() {
					if r := recover(); r != nil {
						reject.Invoke(js.Global().Get("Error").New("engine operation failed"))
					}
				}()
				out, err := handler(input)
				if err != nil {
					reject.Invoke(js.Global().Get("Error").New(err.Error()))
					return
				}
				resolve.Invoke(string(out))
			}()
			return nil
		})
		// The Promise constructor runs the executor synchronously.
		defer executor.Release()
		return js.Global().Get("Promise").New(executor)
	})
}
