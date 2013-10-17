package rtmfp

import (
	"testing"
	"time"
)

func TestSession(t *testing.T) {

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

	initiator.send_ping(initiator.other_addr)

	time.Sleep(100 * time.Millisecond) //100ms is enough to run the logic.
}
