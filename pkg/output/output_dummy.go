package output

import (
	"github.com/buger/goreplay/pkg"
	"github.com/buger/goreplay/pkg/input"
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
func (i *DummyOutput) PluginWrite(msg *pkg.Message) (int, error) {
	var n, nn int
	var err error
	n, err = os.Stdout.Write(msg.Meta)
	nn, err = os.Stdout.Write(msg.Data)
	n += nn
	nn, err = os.Stdout.Write(input.payloadSeparatorAsBytes)
	n += nn

	return n, err
}

func (i *DummyOutput) String() string {
	return "Dummy Output"
}
