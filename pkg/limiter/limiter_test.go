// +build !race

package limiter

import (
	"github.com/reoring/goreplay/pkg/emitter"
	"github.com/reoring/goreplay/pkg/input"
	output2 "github.com/reoring/goreplay/pkg/output"
	"github.com/reoring/goreplay/pkg/plugin"
	"github.com/reoring/goreplay/pkg/settings"
	"sync"
	"testing"
)

func TestOutputLimiter(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := input.NewTestInput()
	output := NewLimiter(output2.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	}), "10")
	wg.Add(10)

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.PluginReader{input},
		Outputs: []plugin.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.NewEmitter()
	go emitter.Start(plugins, settings.Settings.Middleware)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func TestInputLimiter(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := NewLimiter(input.NewTestInput(), "10")
	output := output2.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	})
	wg.Add(10)

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.PluginReader{input},
		Outputs: []plugin.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.NewEmitter()
	go emitter.Start(plugins, settings.Settings.Middleware)

	for i := 0; i < 100; i++ {
		input.(*Limiter).plugin.(*input.TestInput).EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

// Should limit all requests
func TestPercentLimiter1(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := input.NewTestInput()
	output := NewLimiter(output2.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	}), "0%")

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.PluginReader{input},
		Outputs: []plugin.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.NewEmitter()
	go emitter.Start(plugins, settings.Settings.Middleware)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()
}

// Should not limit at all
func TestPercentLimiter2(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := input.NewTestInput()
	output := NewLimiter(output2.NewTestOutput(func(*plugin.Message) {
		wg.Done()
	}), "100%")
	wg.Add(100)

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.PluginReader{input},
		Outputs: []plugin.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.NewEmitter()
	go emitter.Start(plugins, settings.Settings.Middleware)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()
}
