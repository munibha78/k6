package js

import (
	"github.com/loadimpact/speedboat/runner"
	"github.com/robertkrimen/otto"
	"net/http"
	"time"
)

type JSRunner struct {
	BaseVM *otto.Otto
	Script *otto.Script
}

func New() (r *JSRunner, err error) {
	r = &JSRunner{}

	// Create a base VM
	r.BaseVM = otto.New()

	// Bridge basic functions
	r.BaseVM.Set("sleep", jsSleepFactory(time.Sleep))
	r.BaseVM.Set("get", jsHTTPGetFactory(r.BaseVM, http.Get))

	return r, nil
}

func (r *JSRunner) Load(filename, src string) (err error) {
	r.Script, err = r.BaseVM.Compile(filename, src)
	return err
}

func (r *JSRunner) RunVU() <-chan runner.Result {
	out := make(chan runner.Result)

	go func() {
		defer close(out)

		vm := r.BaseVM.Copy()
		for res := range r.RunIteration(vm) {
			out <- res
		}
	}()

	return out
}

func (r *JSRunner) RunIteration(vm *otto.Otto) <-chan runner.Result {
	out := make(chan runner.Result)

	go func() {
		defer close(out)
		defer func() {
			if err := recover(); err != nil {
				out <- runner.Result{
					Type:  "error",
					Error: err.(error),
				}
			}
		}()

		// Log has to be bridged here, as it needs a reference to the channel
		vm.Set("log", jsLogFactory(func(text string) {
			out <- runner.Result{
				Type: "log",
				LogEntry: runner.LogEntry{
					Time: time.Now(),
					Text: text,
				},
			}
		}))

		startTime := time.Now()
		_, err := vm.Run(r.Script)
		duration := time.Since(startTime)

		if err != nil {
			out <- runner.Result{
				Type:  "error",
				Error: err,
			}
		}

		out <- runner.Result{
			Type: "metric",
			Metric: runner.Metric{
				Time:     time.Now(),
				Duration: duration,
			},
		}
	}()

	return out
}
