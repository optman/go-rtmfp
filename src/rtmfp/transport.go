package rtmfp

import (
	"errors"
	"io"
	"time"

	//	"encoding/hex"
	"fmt"
)

type StreamHandler func(s *BiStream, addr string) bool

type Transport struct {
	socket    *socket_bin
	handshake *handshake

	stream_handler StreamHandler

	in_chan, out_chan *noisy_chan
}

func (self *Transport) SetStreamHandler(h StreamHandler) {
	self.stream_handler = h
}

func (self *Transport) SetInChannelParam(delay time.Duration, capacity, lose_rate, speed int) {
	nc := &noisy_chan{
		in:        self.socket.out,
		lose_rate: lose_rate,
		delay:     delay,
		capacity:  capacity,
		speed:     speed,
	}
	nc.open()
	self.handshake.in = nc.out
	self.in_chan = nc
}

func (self *Transport) SetOutChannelParam(delay time.Duration, capacity, lose_rate, speed int) {
	nc := &noisy_chan{
		in:        self.handshake.out,
		lose_rate: lose_rate,
		delay:     delay,
		capacity:  capacity,
		speed:     speed,
	}
	nc.open()
	self.socket.in = nc.out
	self.out_chan = nc
}

func (self *Transport) SetFlowRecvBufSize(size int) {
	max_recv_buf_size = uint(size)
}

func (self *Transport) SetTimeCritical(tc bool) {
	is_time_critical = tc
}

func (self *Transport) SetFastGrow(fg bool) {
	fastgrow_allowed = fg
}

func (self *Transport) Open(localAddr string, pseudoId []byte) (err error) {
	self.socket = &socket_bin{}
	err = self.socket.open(localAddr)
	if err != nil {
		return err
	}

	self.handshake = &handshake{
		in:  self.socket.out,
		out: self.socket.in,
	}

	if len(pseudoId) < len(self.handshake.pseudo_id) {
		copy(self.handshake.pseudo_id[0:len(pseudoId)], pseudoId)
	} else {
		copy(self.handshake.pseudo_id[0:len(self.handshake.pseudo_id)], pseudoId)
	}

	self.handshake.create_passive_session = func(addr string, nearid []byte) (s *session, err error) {

		s = self.handshake.new_session()
		s.passive_open()

		//NOTE: nearid is not the same as peerid.

		stream := &bi_stream{ /*name: hex.EncodeToString(nearid)*/}
		err = stream.passive_open(s /*hex.EncodeToString(self.Peerid())*/, "WHATEVER")
		if err != nil {
			return nil, err
		}

		if self.stream_handler == nil {
			panic("SetStreamHandler() before Open()!")
			return nil, errors.New("StreamHandler not set.")
		}

		if self.stream_handler(&BiStream{stream: stream}, addr) {
			return s, nil
		} else {
			return nil, errors.New("stream handler return false.")
		}
	}

	return self.handshake.open()
}

func (self *Transport) Close() {
	self.socket.close()
	self.handshake.close()
}

func (self *Transport) CreateBiStream(dstAddr string, dstPeerid []byte) (*BiStream, error) {

	s, err := self.handshake.create_session(dstAddr, dstPeerid)
	if err != nil {
		return nil, err
	}

	stream := &bi_stream{ /*name: hex.EncodeToString(dstPeerid)*/}
	err = stream.active_open(s /*hex.EncodeToString(self.Peerid())*/, "WHATEVER")
	if err != nil {
		return nil, err
	}

	return &BiStream{stream: stream}, nil
}

func (self *Transport) Peerid() []byte {
	return self.handshake.peerid()
}

func (self *Transport) LocalAddr() string {
	return self.socket.local_addr().String()
}

func (self *Transport) DumpState(w io.Writer) {
	fmt.Fprintln(w, "[IN_QUEUE]")
	dump_queue_state(self.in_chan, w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "[OUT_QUEUE]")
	dump_queue_state(self.out_chan, w)
}

func dump_queue_state(nc *noisy_chan, w io.Writer) {
	if nc != nil {

		load := 0
		if nc.capacity != 0 {
			load = nc.packets.Len() * 100 / nc.capacity
		}

		drop := 0
		if nc.rx_count != 0 {
			drop = nc.drop_count * 100 / nc.rx_count
		}

		fmt.Fprintf(w, "speed: %d\tdelay: %v\tcap: %d\tqueued: %d\tload: %d%%\trx: %d\tdrop: %d%%\ttx: %d\n",
			nc.speed, nc.delay, nc.capacity, nc.packets.Len(),
			load, nc.rx_count, drop, nc.tx_count)
	} else {
		fmt.Fprintf(w, "n/a")
	}
}

type BiStream struct {
	stream *bi_stream
}

func (self *BiStream) Close() {
	self.stream.close()
}

func (self *BiStream) Send(data []byte) error {
	return self.stream.send(data)
}

func (self *BiStream) Recv() ([]byte, error) {
	return self.stream.recv()
}

func (self *BiStream) DumpState(w io.Writer) {
	self.stream.dump_state(w)
}
