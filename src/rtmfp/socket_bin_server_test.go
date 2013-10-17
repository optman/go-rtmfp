package rtmfp

import (
	"fmt"
	"testing"
	"time"
)

func create_server_socket_bin() (*socket_bin, *handshake) {

	bin := socket_bin{}
	bin.open(":1909")

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	return &bin, &hs
}

func TestServerSocketBin(t *testing.T) {

	bin, hs := create_server_socket_bin()

	hs.create_passive_session = func(addr string, peerid []byte) (*session, error) {
		s := hs.new_session()
		s.passive_open()

		s.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

			recv_flow, _ := s.new_recv_flow(flowid)
			send_flow, _ := s.new_send_flow(0, nil)

			go func() {

				for {
					data, err := recv_flow.recv()

					if err != nil {
						fmt.Println(err)
						break
					}

					send_flow.send(data)
				}
			}()

			return recv_flow, nil
		}

		return s, nil
	}

	//time.Sleep(1000000 * time.Second)
	time.Sleep(0 * time.Second)

	bin.close()
	hs.close()
}
