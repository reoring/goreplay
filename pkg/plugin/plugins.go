package plugin

import (
	"github.com/reoring/goreplay/pkg/input"
	"github.com/reoring/goreplay/pkg/limiter"
	"github.com/reoring/goreplay/pkg/output"
	"github.com/reoring/goreplay/pkg/settings"
	"reflect"
	"strings"
)

// Message represents data across plugins
type Message struct {
	Meta []byte // metadata
	Data []byte // actual data
}

// PluginReader is an interface for input plugins
type PluginReader interface {
	PluginRead() (msg *Message, err error)
}

// PluginWriter is an interface for output plugins
type PluginWriter interface {
	PluginWrite(msg *Message) (n int, err error)
}

// PluginReadWriter is an interface for plugins that support reading and writing
type PluginReadWriter interface {
	PluginReader
	PluginWriter
}

// InOutPlugins struct for holding references to plugins
type InOutPlugins struct {
	Inputs  []PluginReader
	Outputs []PluginWriter
	All     []interface{}
}

// extractLimitOptions detects if plugin get called with limiter support
// Returns address and limit
func extractLimitOptions(options string) (string, string) {
	split := strings.Split(options, "|")

	if len(split) > 1 {
		return split[0], split[1]
	}

	return split[0], ""
}

// Automatically detects type of plugin and initialize it
//
// See this article if curious about reflect stuff below: http://blog.burntsushi.net/type-parametric-functions-golang
func (plugins *InOutPlugins) registerPlugin(constructor interface{}, options ...interface{}) {
	var path, limit string
	vc := reflect.ValueOf(constructor)

	// Pre-processing options to make it work with reflect
	vo := []reflect.Value{}
	for _, oi := range options {
		vo = append(vo, reflect.ValueOf(oi))
	}

	if len(vo) > 0 {
		// Removing limit options from path
		path, limit = extractLimitOptions(vo[0].String())

		// Writing value back without limiter "|" options
		vo[0] = reflect.ValueOf(path)
	}

	// Calling our constructor with list of given options
	plugin := vc.Call(vo)[0].Interface()

	if limit != "" {
		plugin = limiter.NewLimiter(plugin, limit)
	}

	// Some of the output can be Readers as well because return responses
	if r, ok := plugin.(PluginReader); ok {
		plugins.Inputs = append(plugins.Inputs, r)
	}

	if w, ok := plugin.(PluginWriter); ok {
		plugins.Outputs = append(plugins.Outputs, w)
	}
	plugins.All = append(plugins.All, plugin)
}

// NewPlugins specify and initialize all available plugins
func NewPlugins() *InOutPlugins {
	plugins := new(InOutPlugins)

	for _, options := range settings.Settings.InputDummy {
		plugins.registerPlugin(input.NewDummyInput, options)
	}

	for range settings.Settings.OutputDummy {
		plugins.registerPlugin(output.NewDummyOutput)
	}

	if settings.Settings.OutputStdout {
		plugins.registerPlugin(output.NewDummyOutput)
	}

	if settings.Settings.OutputNull {
		plugins.registerPlugin(output.NewNullOutput)
	}

	for _, options := range settings.Settings.InputRAW {
		plugins.registerPlugin(input.NewRAWInput, options, settings.Settings.RAWInputConfig)
	}

	for _, options := range settings.Settings.InputTCP {
		plugins.registerPlugin(input.NewTCPInput, options, &settings.Settings.InputTCPConfig)
	}

	for _, options := range settings.Settings.OutputTCP {
		plugins.registerPlugin(output.NewTCPOutput, options, &settings.Settings.OutputTCPConfig)
	}

	for _, options := range settings.Settings.InputFile {
		plugins.registerPlugin(input.NewFileInput, options, settings.Settings.InputFileLoop, settings.Settings.InputFileReadDepth, settings.Settings.InputFileMaxWait, settings.Settings.InputFileDryRun)
	}

	for _, path := range settings.Settings.OutputFile {
		if strings.HasPrefix(path, "s3://") {
			plugins.registerPlugin(output.NewS3Output, path, &settings.Settings.OutputFileConfig)
		} else {
			plugins.registerPlugin(output.NewFileOutput, path, &settings.Settings.OutputFileConfig)
		}
	}

	for _, options := range settings.Settings.InputHTTP {
		plugins.registerPlugin(input.NewHTTPInput, options)
	}

	// If we explicitly set Host header http output should not rewrite it
	// Fix: https://github.com/reoring/gor/issues/174
	for _, header := range settings.Settings.ModifierConfig.Headers {
		if header.Name == "Host" {
			settings.Settings.OutputHTTPConfig.OriginalHost = true
			break
		}
	}

	for _, options := range settings.Settings.OutputHTTP {
		plugins.registerPlugin(output.NewHTTPOutput, options, &settings.Settings.OutputHTTPConfig)
	}

	for _, options := range settings.Settings.OutputBinary {
		plugins.registerPlugin(output.NewBinaryOutput, options, &settings.Settings.OutputBinaryConfig)
	}

	if settings.Settings.OutputKafkaConfig.Host != "" && settings.Settings.OutputKafkaConfig.Topic != "" {
		plugins.registerPlugin(output.NewKafkaOutput, "", &settings.Settings.OutputKafkaConfig, &settings.Settings.KafkaTLSConfig)
	}

	if settings.Settings.InputKafkaConfig.Host != "" && settings.Settings.InputKafkaConfig.Topic != "" {
		plugins.registerPlugin(input.NewKafkaInput, "", &settings.Settings.InputKafkaConfig, &settings.Settings.KafkaTLSConfig)
	}

	return plugins
}
