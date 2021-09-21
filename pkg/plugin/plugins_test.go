package plugin

import (
	"github.com/reoring/goreplay/pkg/input"
	"github.com/reoring/goreplay/pkg/limiter"
	"github.com/reoring/goreplay/pkg/output"
	"github.com/reoring/goreplay/pkg/settings"
	"testing"
)

func TestPluginsRegistration(t *testing.T) {
	settings.Settings.InputDummy = settings.MultiOption{"[]"}
	settings.Settings.OutputDummy = settings.MultiOption{"[]"}
	settings.Settings.OutputHTTP = settings.MultiOption{"www.example.com|10"}
	settings.Settings.InputFile = settings.MultiOption{"/dev/null"}

	plugins := NewPlugins()

	if len(plugins.Inputs) != 3 {
		t.Errorf("Should be 3 inputs got %d", len(plugins.Inputs))
	}

	if _, ok := plugins.Inputs[0].(*input.DummyInput); !ok {
		t.Errorf("First input should be DummyInput")
	}

	if _, ok := plugins.Inputs[1].(*input.FileInput); !ok {
		t.Errorf("Second input should be FileInput")
	}

	if len(plugins.Outputs) != 2 {
		t.Errorf("Should be 2 output %d", len(plugins.Outputs))
	}

	if _, ok := plugins.Outputs[0].(*output.DummyOutput); !ok {
		t.Errorf("First output should be DummyOutput")
	}

	if l, ok := plugins.Outputs[1].(*limiter.Limiter); ok {
		if _, ok := l.plugin.(*output.HTTPOutput); !ok {
			t.Errorf("HTTPOutput should be wrapped in limiter")
		}
	} else {
		t.Errorf("Second output should be Limiter")
	}

}