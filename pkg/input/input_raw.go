package input

import (
	"context"
	"fmt"
	"github.com/reoring/goreplay/pkg/plugin"
	"github.com/reoring/goreplay/pkg/protocol"
	"github.com/reoring/goreplay/pkg/settings"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reoring/goreplay/capture"
	"github.com/reoring/goreplay/proto"
	"github.com/reoring/goreplay/size"
	"github.com/reoring/goreplay/tcp"
)

// RAWInputConfig represents configuration that can be applied on raw input
type RAWInputConfig struct {
	capture.PcapOptions
	Expire          time.Duration      `json:"input-raw-expire"`
	CopyBufferSize  size.Size          `json:"copy-buffer-size"`
	Engine          capture.EngineType `json:"input-raw-engine"`
	TrackResponse   bool               `json:"input-raw-track-response"`
	Protocol        tcp.TCPProtocol    `json:"input-raw-protocol"`
	RealIPHeader    string             `json:"input-raw-realip-header"`
	Stats           bool               `json:"input-raw-stats"`
	AllowIncomplete bool               `json:"input-raw-allow-incomplete"`
	quit            chan bool          // Channel used only to indicate goroutine should shutdown
	host            string
	ports           []uint16
}

// RAWInput used for intercepting traffic for given address
type RAWInput struct {
	sync.Mutex
	RAWInputConfig
	messageStats   []tcp.Stats
	listener       *capture.Listener
	messageParser  *tcp.MessageParser
	cancelListener context.CancelFunc
	closed         bool
}

// NewRAWInput constructor for RAWInput. Accepts raw input config as arguments.
func NewRAWInput(address string, config RAWInputConfig) (i *RAWInput) {
	i = new(RAWInput)
	i.RAWInputConfig = config
	i.quit = make(chan bool)

	host, _ports, err := net.SplitHostPort(address)
	if err != nil {
		log.Fatalf("input-raw: error while parsing address: %s", err)
	}

	var ports []uint16
	if _ports != "" {
		portsStr := strings.Split(_ports, ",")

		for _, portStr := range portsStr {
			port, err := strconv.Atoi(strings.TrimSpace(portStr))
			if err != nil {
				log.Fatalf("parsing port error: %v", err)
			}
			ports = append(ports, uint16(port))

		}
	}

	i.host = host
	i.ports = ports

	i.listen(address)

	return
}

// PluginRead reads meassage from this plugin
func (i *RAWInput) PluginRead() (*plugin.Message, error) {
	var msgTCP *tcp.Message
	var msg plugin.Message
	select {
	case <-i.quit:
		return nil, ErrorStopped
	case msgTCP = <-i.listener.Messages():
		msg.Data = msgTCP.Data()
	}

	var msgType byte = protocol.ResponsePayload
	if msgTCP.Direction == tcp.DirIncoming {
		msgType = protocol.RequestPayload
		if i.RealIPHeader != "" {
			msg.Data = proto.SetHeader(msg.Data, []byte(i.RealIPHeader), []byte(msgTCP.SrcAddr))
		}
	}
	msg.Meta = protocol.PayloadHeader(msgType, msgTCP.UUID(), msgTCP.Start.UnixNano(), msgTCP.End.UnixNano()-msgTCP.Start.UnixNano())

	// to be removed....
	if msgTCP.Truncated {
		settings.Debug(2, "[INPUT-RAW] message truncated, increase copy-buffer-size")
	}
	// to be removed...
	if msgTCP.TimedOut {
		settings.Debug(2, "[INPUT-RAW] message timeout reached, increase input-raw-expire")
	}
	if i.Stats {
		stat := msgTCP.Stats
		go i.addStats(stat)
	}
	msgTCP = nil
	return &msg, nil
}

func (i *RAWInput) listen(address string) {
	var err error
	i.listener, err = capture.NewListener(i.host, i.ports, "", i.Engine, i.Protocol, i.TrackResponse, i.Expire, i.AllowIncomplete)
	if err != nil {
		log.Fatal(err)
	}
	i.listener.SetPcapOptions(i.PcapOptions)
	err = i.listener.Activate()
	if err != nil {
		log.Fatal(err)
	}

	var ctx context.Context
	ctx, i.cancelListener = context.WithCancel(context.Background())
	errCh := i.listener.ListenBackground(ctx)
	<-i.listener.Reading
	settings.Debug(1, i)
	go func() {
		<-errCh // the listener closed voluntarily
		i.Close()
	}()
}

func (i *RAWInput) String() string {
	return fmt.Sprintf("Intercepting traffic from: %s:%s", i.host, strings.Join(strings.Fields(fmt.Sprint(i.ports)), ","))
}

// GetStats returns the stats so far and reset the stats
func (i *RAWInput) GetStats() []tcp.Stats {
	i.Lock()
	defer func() {
		i.messageStats = []tcp.Stats{}
		i.Unlock()
	}()
	return i.messageStats
}

// Close closes the input raw listener
func (i *RAWInput) Close() error {
	i.Lock()
	defer i.Unlock()
	if i.closed {
		return nil
	}
	i.cancelListener()
	close(i.quit)
	i.closed = true
	return nil
}

func (i *RAWInput) addStats(mStats tcp.Stats) {
	i.Lock()
	if len(i.messageStats) >= 10000 {
		i.messageStats = []tcp.Stats{}
	}
	i.messageStats = append(i.messageStats, mStats)
	i.Unlock()
}
