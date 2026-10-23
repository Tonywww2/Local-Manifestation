package client

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
)

func TestIndependentInputClockSequenceAndDodgeLatch(t *testing.T) {
	for _, rate := range []int{30, 60} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			clientConn, peerConn := net.Pipe()
			l := newLink(clientConn, rate)
			defer l.Close()
			defer peerConn.Close()
			requests := make(chan protocol.Request, 128)
			go func() {
				defer close(requests)
				d := json.NewDecoder(peerConn)
				for {
					var r protocol.Request
					if d.Decode(&r) != nil {
						return
					}
					requests <- r
				}
			}()
			l.SetInput(model.Input{Move: model.Vec{X: 1}, Dodge: true})
			l.SetInput(model.Input{Move: model.Vec{X: 1}})
			l.Send(protocol.Request{Seq: 999, Kind: "attribute", Node: "strength"})
			deadline := time.NewTimer(1200 * time.Millisecond)
			defer deadline.Stop()
			var sequence uint64
			inputs, dodges, stops, commands := 0, 0, 0, 0
		loop:
			for {
				select {
				case r := <-requests:
					if r.Seq != sequence+1 {
						t.Fatal("writer lost request order", r.Seq, sequence)
					}
					sequence = r.Seq
					if r.Kind == "input" {
						inputs++
						if r.Input.Dodge {
							dodges++
						}
						if r.Input.Move == (model.Vec{}) {
							stops++
						}
					} else if r.Kind != "ping" {
						commands++
					}
				case <-deadline.C:
					break loop
				}
			}
			// No Update or Draw calls occurred during this interval.
			if inputs < rate || inputs > rate*3/2 || dodges != 1 || stops == 0 || commands != 1 {
				t.Fatal(rate, inputs, dodges, stops, commands)
			}
			if hz := l.InputHz(); hz < float64(rate)*.8 || hz > float64(rate)*1.2 {
				t.Fatal("incorrect input meter", hz)
			}
			l.ClearInput()
		})
	}
}

func TestLatencyAcceptsDelayedPongAndExpires(t *testing.T) {
	l := &Link{}
	now := time.Now()
	stamp := perf.Start()
	// A pong may arrive after a newer ping has already been sent (> 1s RTT).
	l.probe = 2
	l.probes[1] = latencyProbe{1, now.Add(-1500 * time.Millisecond), stamp}
	l.probes[2] = latencyProbe{2, now, stamp}
	l.acceptPong(1, now)
	if _, valid := l.Latency(now); !valid || l.acceptedProbe != 1 {
		t.Fatal("delayed pong was discarded")
	}
	l.acceptPong(2, now.Add(time.Millisecond))
	if l.acceptedProbe != 2 {
		t.Fatal("new pong was not accepted")
	}
	l.acceptPong(1, now.Add(2*time.Millisecond))
	if l.latencyAt != now.Add(time.Millisecond) {
		t.Fatal("duplicate/reordered pong replaced latency")
	}
	if _, valid := l.Latency(now.Add(4 * time.Second)); valid {
		t.Fatal("stale latency still displayed")
	}
	l.acceptPong(999, now)
	if l.acceptedProbe != 2 {
		t.Fatal("unknown probe was accepted")
	}
}

func TestPingPongUsesOrderedConnectionWithoutUIQueue(t *testing.T) {
	clientConn, peerConn := net.Pipe()
	l := newLink(clientConn, 60)
	defer l.Close()
	defer peerConn.Close()
	errCh := make(chan error, 1)
	go func() {
		var request protocol.Request
		if err := json.NewDecoder(peerConn).Decode(&request); err != nil {
			errCh <- err
			return
		}
		if request.Kind != "ping" || request.Probe != 1 || request.Seq != 1 {
			errCh <- fmt.Errorf("unexpected probe %+v", request)
			return
		}
		time.Sleep(10 * time.Millisecond)
		errCh <- json.NewEncoder(peerConn).Encode(protocol.Message{Kind: "pong", Probe: request.Probe})
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ping/pong stalled")
	}
	deadline := time.Now().Add(time.Second)
	for {
		latency, valid := l.Latency(time.Now())
		if valid {
			if latency < 10*time.Millisecond {
				t.Fatal("RTT omitted peer delay", latency)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pong did not update latency")
		}
		time.Sleep(time.Millisecond)
	}
	if len(l.Messages) != 0 {
		t.Fatal("pong entered gameplay/UI queue")
	}
}
