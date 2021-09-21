// +build !race

package limiter

import (
	"github.com/reoring/goreplay/pkg/core"
	"github.com/reoring/goreplay/pkg/emitter"
	"github.com/reoring/goreplay/pkg/settings"
	"sync"
	"testing"
)

func TestOutputLimiter(t *testing.T) {
	wg := new(sync.WaitGroup)

	input := core.NewTestInput()
	output := NewLimiter(core.NewTestOutput(func(*core.Message) {
		wg.Done()
	}), "10")
	wg.Add(10)

	plugins := &core.InOutPlugins{
		Inputs:  []core.PluginReader{input},
		Outputs: []core.PluginWriter{output},
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

	input := NewLimiter(core.NewTestInput(), "10")
	output := core.NewTestOutput(func(*core.Message) {
		wg.Done()
	})
	wg.Add(10)

	plugins := &core.InOutPlugins{
		Inputs:  []core.PluginReader{input},
		Outputs: []core.PluginWriter{output},
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

	input := core.NewTestInput()
	output := NewLimiter(core.NewTestOutput(func(*core.Message) {
		wg.Done()
	}), "0%")

	plugins := &core.InOutPlugins{
		Inputs:  []core.PluginReader{input},
		Outputs: []core.PluginWriter{output},
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

	input := core.NewTestInput()
	output := NewLimiter(core.NewTestOutput(func(*core.Message) {
		wg.Done()
	}), "100%")
	wg.Add(100)

	plugins := &core.InOutPlugins{
		Inputs:  []core.PluginReader{input},
		Outputs: []core.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	emitter := emitter.NewEmitter()
	go emitter.Start(plugins, settings.Settings.Middleware)

	for i := 0; i < 100; i++ {
		input.EmitGET()
	}

	wg.Wait()
}
