package rtmfp

import (
	"fmt"
	"testing"
	"time"
)

func create_netstream_sessions() (init, resp *session) {

	chan_a := make(chan *network_packet, network_packet_chan_default_buffer_size)
	chan_b := make(chan *network_packet, network_packet_chan_default_buffer_size)

	initiator := session{
		in:        chan_a,
		out:       chan_b,
		sessionid: 1,
	}

	responder := session{
		in:        chan_b,
		out:       chan_a,
		sessionid: 2,
	}

	responder.passive_open()
	initiator.active_open("", nil, nil)

	return &initiator, &responder
}

func TestNetStream(t *testing.T) {

	initiator, responder := create_netstream_sessions()

	handler := "foo"
	msg := "hello world"

	done := make(chan bool)

	send_stream := &net_stream{
		session: initiator,
	}
	send_stream_id, _ := send_stream.active_open()

	initiator.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

		rel_flowid := read_vlu_option(options, 0xa, 0)

		if rel_flowid == send_stream_id {
			//fmt.Printf("recv response flow %d\n", flowid)
			send_stream.attach_flow(flowid)

			go func() {
				for {
					cmd, param, err := send_stream.recv()
					if err != nil || cmd == "" {
						break
					}

					fmt.Printf("#recv cmd:%s\n", cmd)

					if cmd == "onStatus" {
						obj := param.(map[string]interface{})
						fmt.Printf("onStatus:%s\n", obj["code"])
					} else if cmd == "play" {
						send_stream.send(handler, msg)
					}
				}
			}()

			return send_stream.recvFlow, nil

		} else {
			t.Fatal("not expect this flow!")

			return nil, nil
		}
	}

	responder.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

		recv_stream := &net_stream{
			session: responder,
		}

		err := recv_stream.passive_open(flowid)

		if err == nil {

			go func() {

				for {
					cmd, param, err := recv_stream.recv()
					if err != nil {
						break
					}
					fmt.Printf("###recv cmd:%s\n", cmd)

					if cmd == "publish" {
						recv_stream.play(param.(string))
					} else if cmd == "onStatus" {
						obj := param.(map[string]interface{})
						fmt.Printf("onStatus:%s\n", obj["code"])
					} else {

						if cmd != handler || msg != param {
							t.Fatal("msg not match!")
						}

						done <- true
						break
					}
				}
			}()

		}

		return recv_stream.recvFlow, err
	}

	send_stream.publish("OUT")

	//send_stream.send(handler, msg)

	select {
	case <-done:
	case <-time.After(1 * time.Second): //1s is enough to run the logic.
		t.Fatal()
	}

	initiator.close()
	responder.close()
}
