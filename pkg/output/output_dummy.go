package output

import (
	"github.com/reoring/goreplay/pkg/input"
	"github.com/reoring/goreplay/pkg/plugin"
	"os"
)

// DummyOutput used for debugging, prints all incoming requests
type DummyOutput struct {
}

// NewDummyOutput constructor for DummyOutput
func NewDummyOutput() (di *DummyOutput) {
	di = new(DummyOutput)

	return
}

// PluginWrite writes message to this plugin
func (i *DummyOutput) PluginWrite(msg *plugin.Message) (int, error) {
	var n, nn int
	var err error
	n, err = os.Stdout.Write(msg.Meta)
	nn, err = os.Stdout.Write(msg.Data)
	n += nn
	nn, err = os.Stdout.Write(input.PayloadSeparatorAsBytes)
	n += nn

	return n, err
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
