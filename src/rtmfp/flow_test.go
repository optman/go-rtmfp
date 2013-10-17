package rtmfp

import (
	"bytes"
	"crypto/rand"
	//	"fmt"
	"testing"
	"time"
)

func create_sessions() (init, resp *session, err error) {

	chan_a := make(chan *network_packet, network_packet_chan_default_buffer_size)
	chan_b := make(chan *network_packet, network_packet_chan_default_buffer_size)

	chan_a_x := &noisy_chan{
		in:              chan_a,
		lose_rate:       50,
		delay:           10 * time.Millisecond,
		capacity:        10,
		disorder:        true,
		max_packet_size: 1500,
	}
	chan_a_x.open()

	initiator := session{
		in:        chan_a_x.out,
		out:       chan_b,
		sessionid: 1,
	}

	responder := session{
		in:        chan_b,
		out:       chan_a_x.in,
		sessionid: 2,
	}

	responder.passive_open()
	err = initiator.active_open("", nil, nil)

	return &initiator, &responder, err
}

func create_random_msgs() (msgs [][]byte) {

	sizes := []int{100, 1024 * 10, 1024 * /*100*/ 80} /*when noisy_chan enable disorder, msg size should not equal or too near max buffer size.*/

	msgs = make([][]byte, len(sizes))
	for i, size := range sizes {
		msgs[i] = make([]byte, size)
		rand.Read(msgs[i])
	}

	return
}

func TestFlow(t *testing.T) {

	initiator, responder, err := create_sessions()

	if err != nil {
		t.Fatalf(err.Error())
	}

	msgs := create_random_msgs()

	done := make(chan bool)

	responder.create_recv_flow = func(signature []byte, flowid uint) (*recv_flow, error) {

		flow, err := responder.new_recv_flow(flowid)
		if err == nil {

			go func() {

				for i := 0; i < len(msgs); i++ {

					buf, _ := flow.recv()
					if !bytes.Equal(buf, msgs[i]) {
						t.Fatalf("msg %d not match!(%d  %d)\n", i, len(buf), len(msgs[i]))
					}
				}

				done <- true

			}()

		}

		return flow, err
	}

	send_flow, _ := initiator.new_send_flow(0, nil)

	for _, msg := range msgs {
		send_flow.send(msg)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout!")
	}

	initiator.close()
	responder.close()
}
