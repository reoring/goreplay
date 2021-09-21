package input

import (
	"bytes"
	"github.com/reoring/goreplay/pkg"
	"github.com/reoring/goreplay/pkg/output"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reoring/goreplay/capture"
	"github.com/reoring/goreplay/proto"
	"github.com/reoring/goreplay/tcp"
)

const testRawExpire = time.Millisecond * 200

func TestRAWInputIPv4(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
		return
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())

	var respCounter, reqCounter int64
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        0,
		Protocol:      tcp.ProtocolHTTP,
		TrackResponse: true,
		RealIPHeader:  "X-Real-IP",
	}
	input := NewRAWInput(listener.Addr().String(), conf)

	output := output.NewTestOutput(func(msg *pkg.Message) {
		if msg.Meta[0] == '1' {
			if len(proto.Header(msg.Data, []byte("X-Real-IP"))) == 0 {
				t.Error("Should have X-Real-IP header")
			}
			reqCounter++
		} else {
			respCounter++
		}

		wg.Done()
	})

	plugins := &pkg.InOutPlugins{
		Inputs:  []pkg.PluginReader{input},
		Outputs: []pkg.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	addr := "http://127.0.0.1:" + port
	emitter := pkg.NewEmitter()
	defer emitter.Close()
	go emitter.Start(plugins, pkg.Settings.Middleware)

	// time.Sleep(time.Second)
	for i := 0; i < 1; i++ {
		wg.Add(2)
		_, err = http.Get(addr)

		if err != nil {
			t.Error(err)
			return
		}
	}

	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
}

func TestRAWInputNoKeepAlive(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	origin.SetKeepAlivesEnabled(false)
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())

	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      tcp.ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(":"+port, conf)
	var respCounter, reqCounter int64
	output := output.NewTestOutput(func(msg *pkg.Message) {
		if msg.Meta[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
			wg.Done()
		} else {
			atomic.AddInt64(&respCounter, 1)
			wg.Done()
		}
	})

	plugins := &pkg.InOutPlugins{
		Inputs:  []pkg.PluginReader{input},
		Outputs: []pkg.PluginWriter{output},
	}
	plugins.All = append(plugins.All, input, output)

	addr := "http://127.0.0.1:" + port

	emitter := pkg.NewEmitter()
	go emitter.Start(plugins, pkg.Settings.Middleware)

	for i := 0; i < 10; i++ {
		// request + response
		wg.Add(2)
		_, err = http.Get(addr)
		if err != nil {
			t.Error(err)
			return
		}
	}

	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
	emitter.Close()
}

func TestRAWInputIPv6(t *testing.T) {
	wg := new(sync.WaitGroup)

	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		return
	}
	origin := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go origin.Serve(listener)
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	originAddr := "[::1]:" + port

	var respCounter, reqCounter int64
	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Protocol:      tcp.ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(originAddr, conf)

	output := output.NewTestOutput(func(msg *pkg.Message) {
		if msg.Meta[0] == '1' {
			atomic.AddInt64(&reqCounter, 1)
		} else {
			atomic.AddInt64(&respCounter, 1)
		}
		wg.Done()
	})

	plugins := &pkg.InOutPlugins{
		Inputs:  []pkg.PluginReader{input},
		Outputs: []pkg.PluginWriter{output},
	}

	emitter := pkg.NewEmitter()
	addr := "http://" + originAddr
	go emitter.Start(plugins, pkg.Settings.Middleware)
	for i := 0; i < 10; i++ {
		// request + response
		wg.Add(2)
		_, err = http.Get(addr)
		if err != nil {
			t.Error(err)
			return
		}
	}

	wg.Wait()
	const want = 10
	if reqCounter != respCounter && reqCounter != want {
		t.Errorf("want %d requests and %d responses, got %d requests and %d responses", want, want, reqCounter, respCounter)
	}
	emitter.Close()
}

func TestInputRAWChunkedEncoding(t *testing.T) {
	wg := new(sync.WaitGroup)

	fileContent, _ := ioutil.ReadFile("README.md")

	// Origing and Replay server initialization
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ioutil.ReadAll(r.Body)

		wg.Done()
	}))

	originAddr := strings.Replace(origin.Listener.Addr().String(), "[::]", "127.0.0.1", -1)
	conf := RAWInputConfig{
		Engine:          capture.EnginePcap,
		Expire:          time.Second,
		Protocol:        tcp.ProtocolHTTP,
		TrackResponse:   true,
		AllowIncomplete: true,
	}
	input := NewRAWInput(originAddr, conf)

	replay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)

		if !bytes.Equal(body, fileContent) {
			buf, _ := httputil.DumpRequest(r, true)
			t.Error("Wrong POST body:", string(buf))
		}

		wg.Done()
	}))
	defer replay.Close()

	httpOutput := output.NewHTTPOutput(replay.URL, &output.HTTPOutputConfig{})

	plugins := &pkg.InOutPlugins{
		Inputs:  []pkg.PluginReader{input},
		Outputs: []pkg.PluginWriter{httpOutput},
	}
	plugins.All = append(plugins.All, input, httpOutput)

	emitter := pkg.NewEmitter()
	defer emitter.Close()
	go emitter.Start(plugins, pkg.Settings.Middleware)
	wg.Add(2)

	curl := exec.Command("curl", "http://"+originAddr, "--header", "Transfer-Encoding: chunked", "--header", "Expect:", "--data-binary", "@README.md")
	err := curl.Run()
	if err != nil {
		t.Error(err)
		return
	}

	wg.Wait()
}

func BenchmarkRAWInputWithReplay(b *testing.B) {
	var respCounter, reqCounter, replayCounter uint32
	wg := &sync.WaitGroup{}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		b.Error(err)
		return
	}
	listener0, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		b.Error(err)
		return
	}

	origin := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ab"))
		}),
	}
	go origin.Serve(listener)
	defer origin.Close()
	originAddr := listener.Addr().String()

	replay := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint32(&replayCounter, 1)
			w.Write(nil)
			wg.Done()
		}),
	}
	go replay.Serve(listener0)
	defer replay.Close()
	replayAddr := listener0.Addr().String()

	conf := RAWInputConfig{
		Engine:        capture.EnginePcap,
		Expire:        testRawExpire,
		Protocol:      tcp.ProtocolHTTP,
		TrackResponse: true,
	}
	input := NewRAWInput(originAddr, conf)

	testOutput := output.NewTestOutput(func(msg *pkg.Message) {
		if msg.Meta[0] == '1' {
			reqCounter++
		} else {
			respCounter++
		}
		wg.Done()
	})
	httpOutput := output.NewHTTPOutput("http://"+replayAddr, &output.HTTPOutputConfig{})

	plugins := &pkg.InOutPlugins{
		Inputs:  []pkg.PluginReader{input},
		Outputs: []pkg.PluginWriter{testOutput, httpOutput},
	}

	emitter := pkg.NewEmitter()
	go emitter.Start(plugins, pkg.Settings.Middleware)
	addr := "http://" + originAddr
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(3) // reqCounter + replayCounter + respCounter
		resp, err := http.Get(addr)
		if err != nil {
			wg.Add(-3)
		}
		resp.Body.Close()
	}

	wg.Wait()
	b.ReportMetric(float64(reqCounter), "requests")
	b.ReportMetric(float64(respCounter), "responses")
	b.ReportMetric(float64(replayCounter), "replayed")
	emitter.Close()
}
