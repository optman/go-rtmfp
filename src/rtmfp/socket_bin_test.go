package rtmfp

import (
	//	"fmt"
	"testing"
	"time"
)

func create_socket_bin() (*socket_bin, *handshake) {

	bin := socket_bin{}
	bin.open("127.0.0.1:0")

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	return &bin, &hs
}

func TestSocketBin(t *testing.T) {

	sbs_a, hs_a := create_socket_bin()
	sbs_b, hs_b := create_socket_bin()

	var initiator *session

	hs_b.create_passive_session = func(addr string, peerid []byte) (*session, error) {
		s := hs_b.new_session()
		s.passive_open()
		return s, nil
	}

	var err error
	initiator, err = hs_a.create_session(sbs_b.local_addr().String(), nil)

	if err != nil {
		t.Fatal(err)
	}

	initiator.send_ping(initiator.other_addr)

	//wait for ping reply
	time.Sleep(100 * time.Millisecond)

	sbs_a.close()
	sbs_b.close()

	hs_a.close()
	hs_b.close()

}
