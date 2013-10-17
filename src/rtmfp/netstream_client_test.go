package rtmfp

/*
import (
	"fmt"
	"testing"
	"time"
)

func TestNetStreamClient(t *testing.T) {

	bin := socket_bin{}
	bin.open(":999")

	fmt.Printf("listen:%s\n", bin.local_addr().String())

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	handler := "reciveMessage"
	msg := "hello world"

	var send_stream, recv_stream *net_stream

	create_send_stream := func(s *session, flowid uint) *net_stream {

		ns := &net_stream{
			session: s,
		}

		err := ns.passive_open(flowid)

		if err == nil {

			go func() {

				for {
					cmd, param, _ := ns.recv()
					fmt.Printf("#send_stream recv cmd:%s\n", cmd)

					if cmd == "play" {
						ns.send(handler, []byte(msg))

					} else if cmd == "onStatus" {
						obj := param.(map[string]interface{})
						fmt.Printf("onStatus:%s\n", obj["code"])

					} else if cmd == handler {
						//echo back.
						fmt.Println(string(param.([]byte)))
						send_stream.send(handler, param.([]byte))
					} else {
						fmt.Println(param.(string))
						//break
					}
				}
			}()
		}

		return ns
	}

	create_recv_stream := func(s *session) *net_stream {

		ns := &net_stream{
			session: s,
		}

		ns.active_open()
		ns.play("mystream")

		return ns
	}

	recv_func := func(ns *net_stream) {
		go func() {

			for {
				cmd, param, _ := ns.recv()
				fmt.Printf("#recv_stream recv cmd:%s\n", cmd)

				if cmd == "publish" {
					//ns.send(handler, msg+"\n")
				} else if cmd == "onStatus" {
					obj := param.(map[string]interface{})
					fmt.Printf("onStatus:%s\n", obj["code"])
				} else if cmd == handler {
					//echo back.
					fmt.Println(string(param.([]byte)))
					send_stream.send(handler, param.([]byte))
				} else {
				}
			}
		}()
	}

	hs.create_passive_session = func(addr string, peerid []byte) (*session, error) {
		s := hs.new_session()
		s.passive_open()

		s.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

			if send_stream == nil {
				send_stream = create_send_stream(s, flowid)
				recv_stream = create_recv_stream(s)
				return send_stream.recvFlow, nil
			} else if recv_stream.recvFlow == nil {

				if read_vlu_option(options, 0xa, 0) != recv_stream.sendFlow.flowid {
					t.Fatal("unknown flow!")
				}

				recv_stream.attach_flow(flowid)
				recv_func(recv_stream)
				return recv_stream.recvFlow, nil
			} else {
				t.Fatal("unknow flow!")
			}

			return nil, nil
		}

		return s, nil
	}

	time.Sleep(0 * time.Second) //NOTE: change the timeout to fit your need.

}
*/
