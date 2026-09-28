//go:build js && wasm

// Command craig-wasm exposes the Craig engine to the browser extension. It
// registers Promise-returning functions on globalThis, signals readiness, and
// then blocks; the extension's service worker owns its lifetime.
package main

import (
	"context"
	"syscall/js"

	"github.com/aidandevv/craig-extension/internal/browserapi"
)

func main() {
	host := browserapi.Host{Store: hostStore{js.Global().Get("craigStore")}}
	js.Global().Set("craigAnalyze", promiseFunc(func(in string) ([]byte, error) {
		return host.Analyze(context.Background(), []byte(in))
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
			go func() {
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
