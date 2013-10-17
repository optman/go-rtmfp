package rtmfp

import (
	//	"fmt"
	"testing"
	"time"
)

func create_bis_sessions() (*session, *session) {
	chan_a := make(chan *network_packet, network_packet_chan_default_buffer_size)
	chan_b := make(chan *network_packet, network_packet_chan_default_buffer_size)

	initiator := &session{
		in:        chan_a,
		out:       chan_b,
		sessionid: 1,
	}

	responder := &session{
		in:        chan_b,
		out:       chan_a,
		sessionid: 2,
	}

	responder.passive_open()
	initiator.active_open("", nil, nil)

	return initiator, responder
}

func test_bi_stream(t *testing.T, sa, sb *session) {
	//stream name by the other peer name
	a := &bi_stream{name: "b"}
	b := &bi_stream{name: "a"}

	b.passive_open(sb, "b")
	a.active_open(sa, "a")

	msg := "hello"

	done := make(chan bool)

	go func() {
		data, err := b.recv()
		if err != nil {
			t.Fatal(err)
		}

		//echo
		err = b.send(data)
		if err != nil {
			t.Fatal(err)
		}
	}()

	go func() {
		err := a.send([]byte(msg))
		if err != nil {
			t.Fatal(err)
		}

		data, _ := a.recv()
		if string(data) != msg {
			t.Fatal("msg not match.")
		}

		done <- true
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

}

func TestBiStream(t *testing.T) {

	sa, sb := create_bis_sessions()

	test_bi_stream(t, sa, sb)
}

func create_bi_socket_bin() (*socket_bin, *handshake) {

	bin := socket_bin{}
	bin.open("127.0.0.1:0")

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	return &bin, &hs
}

func TestBiStream2(t *testing.T) {

	_, hs_a := create_bi_socket_bin()
	sbs_b, hs_b := create_bi_socket_bin()

	var initiator, responder *session

	responder_event := make(chan bool, 1)

	hs_b.create_passive_session = func(addr string, peerid []byte) (*session, error) {

		responder = hs_b.new_session()
		responder.passive_open()

		responder_event <- true

		return responder, nil
	}

	var err error
	initiator, err = hs_a.create_session(sbs_b.local_addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-responder_event:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	test_bi_stream(t, initiator, responder)

}
