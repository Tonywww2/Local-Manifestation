package client

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

type receivedMessage struct {
	Message protocol.Message
	At      time.Time
}
type latencyProbe struct {
	id    uint64
	at    time.Time
	stamp perf.Stamp
}
type Link struct {
	diagnostics          perf.Recorder
	pingMu               sync.Mutex
	probe, acceptedProbe uint64
	probes               [8]latencyProbe
	latencyAt            time.Time
	latency              time.Duration
	Messages             chan receivedMessage
	requests             chan protocol.Request
	conn                 net.Conn
	done                 chan struct{}
	once                 sync.Once
	inputMu              sync.Mutex
	input                model.Input
	inputAt              time.Time
	inputActive          bool
	stopInput            bool
	inputMeter           timing.Meter
}

func Dial(addr string, h protocol.Hello, inputRates ...int) (*Link, error) {
	rate := timing.DefaultLogicRate
	if len(inputRates) > 0 {
		rate = inputRates[0]
	}
	if !timing.LogicRate(rate) {
		return nil, protocol.Reject("error.detail", "detail", "input rate must be 30 or 60")
	}
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err = json.NewEncoder(conn).Encode(h); err != nil {
		conn.Close()
		return nil, err
	}
	conn.SetWriteDeadline(time.Time{})
	return newLink(conn, rate), nil
}

func newLink(conn net.Conn, rate int) *Link {
	l := &Link{Messages: make(chan receivedMessage, 32), requests: make(chan protocol.Request, 64), conn: conn, done: make(chan struct{})}
	go func() {
		defer l.Close()
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		for scanner.Scan() {
			decodeStart := perf.Start()
			var m protocol.Message
			if json.Unmarshal(scanner.Bytes(), &m) != nil {
				break
			}
			l.diagnostics.Since("client.network.decode", decodeStart)
			received := receivedMessage{m, time.Now()}
			if m.Kind == "pong" {
				l.acceptPong(m.Probe, received.At)
				continue
			}
			select {
			case l.Messages <- received:
			case <-l.done:
				return
			}
		}
		select {
		case l.Messages <- receivedMessage{protocol.Message{Kind: "disconnected", Code: "notice.disconnected"}, time.Now()}:
		default:
		}
	}()
	go func() {
		defer l.Close()
		encoder := json.NewEncoder(conn)
		ticker := time.NewTicker(time.Second / time.Duration(rate))
		defer ticker.Stop()
		pingTicker := time.NewTicker(time.Second)
		defer pingTicker.Stop()
		var sequence uint64
		send := func(r protocol.Request) bool {
			sequence++
			r.Seq = sequence // One writer orders input and UI commands together.
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			writeStart := perf.Start()
			err := encoder.Encode(r)
			l.diagnostics.Since("client.network.encode_write", writeStart)
			return err == nil
		}
		for {
			select {
			case <-l.done:
				return
			case r := <-l.requests:
				if !send(r) {
					return
				}
			case <-pingTicker.C:
				l.pingMu.Lock()
				l.probe++
				probe := l.probe
				l.probes[probe%uint64(len(l.probes))] = latencyProbe{probe, time.Now(), perf.Start()}
				l.pingMu.Unlock()
				if !send(protocol.Request{Kind: "ping", Probe: probe}) {
					return
				}
			case <-ticker.C:
				l.inputMu.Lock()
				active := l.inputActive || l.stopInput
				in := l.input
				if !l.inputActive || time.Since(l.inputAt) > 250*time.Millisecond {
					in = model.Input{}
				}
				l.input.Dodge = false // Edge input is latched until exactly one send.
				l.stopInput = false
				l.inputMu.Unlock()
				if active {
					if !send(protocol.Request{Kind: "input", Input: in}) {
						return
					}
					l.inputMu.Lock()
					l.inputMeter.Add(1, time.Now())
					l.inputMu.Unlock()
				}
			}
		}
	}()
	return l
}

// SetInput publishes the most recent intent. Sending has its own fixed cadence.
func (l *Link) SetInput(in model.Input) {
	l.inputMu.Lock()
	defer l.inputMu.Unlock()
	in.Dodge = in.Dodge || l.input.Dodge
	l.input = in
	l.inputAt = time.Now()
	l.inputActive = true
	l.stopInput = false
}
func (l *Link) ClearInput() {
	l.inputMu.Lock()
	defer l.inputMu.Unlock()
	if l.inputActive {
		l.stopInput = true
	}
	l.inputActive = false
	l.input = model.Input{}
}
func (l *Link) InputHz() float64 {
	l.inputMu.Lock()
	defer l.inputMu.Unlock()
	return l.inputMeter.Rate()
}
func (l *Link) Send(r protocol.Request) bool {
	select {
	case <-l.done:
		return false
	default:
	}
	select {
	case l.requests <- r:
		return true
	case <-l.done:
		return false
	default:
		return false
	}
}
func (l *Link) Close() { l.once.Do(func() { close(l.done); l.conn.Close() }) }

func (l *Link) acceptPong(probe uint64, now time.Time) {
	l.pingMu.Lock()
	defer l.pingMu.Unlock()
	p := &l.probes[probe%uint64(len(l.probes))]
	if p.id != probe || probe <= l.acceptedProbe || p.at.IsZero() || now.Before(p.at) || now.Sub(p.at) > 8*time.Second {
		return
	}
	l.latency, l.latencyAt, l.acceptedProbe = perf.Elapsed(p.stamp), now, probe
	*p = latencyProbe{}
}
func (l *Link) Latency(now time.Time) (time.Duration, bool) {
	l.pingMu.Lock()
	defer l.pingMu.Unlock()
	return l.latency, !l.latencyAt.IsZero() && now.Sub(l.latencyAt) < 3*time.Second
}
