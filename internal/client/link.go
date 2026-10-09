package client

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

type receivedMessage struct {
	Message protocol.Message
	At      time.Time
}
type Link struct {
	Messages    chan receivedMessage
	requests    chan protocol.Request
	conn        net.Conn
	done        chan struct{}
	once        sync.Once
	inputMu     sync.Mutex
	input       model.Input
	inputAt     time.Time
	inputActive bool
	stopInput   bool
	inputMeter  timing.Meter
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
			var m protocol.Message
			if json.Unmarshal(scanner.Bytes(), &m) != nil {
				break
			}
			received := receivedMessage{m, time.Now()}
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
		var sequence uint64
		send := func(r protocol.Request) bool {
			sequence++
			r.Seq = sequence // One writer orders input and UI commands together.
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			return encoder.Encode(r) == nil
		}
		for {
			select {
			case <-l.done:
				return
			case r := <-l.requests:
				if !send(r) {
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
