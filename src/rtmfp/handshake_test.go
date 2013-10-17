package rtmfp

import (
	"testing"
)

func TestHandshake(t *testing.T) {

	chan1 := make(chan *network_packet, network_packet_chan_default_buffer_size)
	chan2 := make(chan *network_packet, network_packet_chan_default_buffer_size)

	a := &handshake{
		in:  chan1,
		out: chan2,
		//peerid
	}

	b := &handshake{
		in:  chan2,
		out: chan1,
		//peerid
	}

	a.open()
	b.open()

	b.create_passive_session = func(addr string, peerid []byte) (*session, error) {
		s := b.new_session()
		s.passive_open()
		return s, nil
	}

	_, err := a.create_session("", []byte("xyz"))

	if err != nil {
		t.Fatal()
	}
}
