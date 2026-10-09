package client

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"hsmultiplayer/pkg/model"
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
					} else {
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
