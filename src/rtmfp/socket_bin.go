package rtmfp

import (
	//	"fmt"
	"net"
)

var max_udp_packet_size = 2 * 1024

var network_packet_chan_default_buffer_size = 100 * 1000

type network_packet struct {
	data []byte
	addr string
}

type socket_bin struct {
	in, out chan *network_packet
	conn    net.PacketConn
	closed  bool
}

func (self *socket_bin) open(local string) (err error) {

	self.conn, err = net.ListenPacket("udp", local)
	if err != nil {
		return
	}

	self.in = make(chan *network_packet, network_packet_chan_default_buffer_size)
	self.out = make(chan *network_packet, network_packet_chan_default_buffer_size)

	go self.recv()
	go self.dispatch()

	return nil
}

func (self *socket_bin) close() {

	self.closed = true
	self.conn.Close()
}

func (self *socket_bin) dispatch() {
	for {
		p, ok := <-self.in
		if !ok {
			break
		}
		self.send_packet(p)
	}
}

func (self *socket_bin) recv() {

	err_count := 0

	for !self.closed {

		buf := make([]byte, max_udp_packet_size)

		readed_size, raddr, err := self.conn.ReadFrom(buf)

		//NOTE:ReadFrom may mistake fail on windows platform, we should ignore and retry.
		//https://code.google.com/p/go/issues/detail?id=5834

		if err != nil {
			err_count++
			if err_count > 1000 {
				panic("too much error!")
				break
			} else {
				continue
			}
		}

		err_count = 0

		//fmt.Printf("recv from %v, %v bytes\n", raddr, readed_size)

		self.out <- &network_packet{data: buf[:readed_size], addr: raddr.String()}
	}
}

func (self *socket_bin) send_packet(p *network_packet) {

	udp_addr, _ := net.ResolveUDPAddr("udp", p.addr)

	//fmt.Printf("send_packet:%v\n", p.addr)

	self.conn.WriteTo(p.data, udp_addr)
}

func (self *socket_bin) local_addr() net.Addr {
	return self.conn.LocalAddr()
}
