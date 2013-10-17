package rtmfp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	//"encoding/hex"
	"errors"
	//"fmt"
	"strings"
	"time"
)

var ihello_retry = 5 //not exceed 64
var ihello_timeout = 1 * time.Second

var last_sessionid uint32

func new_sessionid() uint32 {
	last_sessionid++
	return last_sessionid
}

type rhello_cb func(srcAddr string, cookie, dh_public []byte)

type create_session_request struct {
	target []byte
	cb     rhello_cb
}

type handshake struct {
	in  chan *network_packet
	out chan *network_packet

	cookie    []byte
	pseudo_id [64]byte

	certificate []byte

	requests map[string]*create_session_request
	sessions map[uint32]*session

	create_passive_session func(addr string, peerid []byte) (*session, error)
}

func (self *handshake) gen_certificate() {

	buf := bytes.NewBuffer(nil)
	buf.Write([]byte("\x01\x0A\x41\x0E"))

	for i := 0; i < len(self.pseudo_id); i++ {
		buf.WriteByte(self.pseudo_id[i])
	}

	buf.Write([]byte("\x02\x15\x02\x02\x15\x05\x02\x15\x0E"))

	self.certificate = buf.Bytes()
}

func (self *handshake) peerid() []byte {
	return gen_peerid_from_cert(self.certificate)
}

func (self *handshake) create_session(addr string, target []byte) (s *session, err error) {
	//fmt.Printf("create_session:%s  %v\n", addr, hex.EncodeToString(target))

	tag := make([]byte, 4)
	rand.Read(tag)

	recv_rhello := false
	continue_chan := make(chan bool, 1)

	var other_addr string
	var cookie_echo, other_dh_public []byte

	self.requests[string(tag)] = &create_session_request{
		target: target,
		cb: func(srcAddr string, cookie, dh_public []byte) {
			other_addr = srcAddr
			cookie_echo = cookie
			other_dh_public = dh_public
			continue_chan <- true
		},
	}

	//send ihello with retry.
forloop:
	for i := 0; i < ihello_retry; i++ {
		self.send_ihello(addr, target, tag)
		select {
		case recv_rhello = <-continue_chan:
			break forloop
		case <-time.After((1 << uint(i)) * ihello_timeout):
		}
	}

	if recv_rhello == false {
		return nil, errors.New("create session fail!(no valid rhello recved.)")
	}

	s = self.new_session()
	err = s.active_open(other_addr, cookie_echo, other_dh_public)

	return s, err
}

func (self *handshake) new_session() *session {
	s := &session{
		sessionid: new_sessionid(),
		in:        make(chan *network_packet, network_packet_chan_default_buffer_size),
		out:       self.out,
	}

	self.sessions[s.sessionid] = s

	return s
}

func (self *handshake) open() error {

	self.requests = make(map[string]*create_session_request)
	self.sessions = make(map[uint32]*session)

	self.cookie = make([]byte, 4)
	rand.Read(self.cookie)

	self.gen_certificate()

	go self.dispatch()

	return nil
}

func (self *handshake) close() {
	//FIXME: clean up.
}

func (self *handshake) dispatch() {
	for {
		p, ok := <-self.in
		if !ok {
			break
		}
		self.recv_packet(p)
	}
}

func (self *handshake) recv_packet(p *network_packet) {

	//fmt.Printf("recv_packet:%s %d\n", p.addr, len(p.data))

	r := bytes.NewBuffer(p.data)

	var scrambleSessionId uint32
	var first32 [2]uint32

	binary.Read(r, binary.BigEndian, &scrambleSessionId)
	binary.Read(r, binary.BigEndian, &first32[0])
	binary.Read(r, binary.BigEndian, &first32[1])

	sessionId := scrambleSessionId ^ first32[0] ^ first32[1]

	if sessionId > 0 {
		//dispatch the established session.
		s, ok := self.sessions[sessionId]
		if ok {
			s.in <- p
		} else {
			//fmt.Printf("unknow sessionid %d!\n", sessionId)
		}

	} else {
		decode_packet(&p.addr, p.data, default_crypto_key, nil, self)
	}
}

func (self *handshake) send_packet(addr string, data []byte) {
	self.out <- &network_packet{addr: addr, data: data}
}

func (self *handshake) send_ihello(addr string, edp, tag []byte) {

	//fmt.Printf("send_ihello(%s)\n", addr)

	chunk_buf := bytes.NewBuffer(nil)

	//0x0a serverurl, 0x0f peerid
	var edp_type uint8
	if strings.Contains(string(edp), "rtmfp://") {
		edp_type = 0x0a
	} else {
		edp_type = 0x0f
	}

	edp_len := uint(len(edp))

	//EndPointDiscriminator
	encode_vlu(chunk_buf, get_vlu_size(1+edp_len)+1+edp_len)
	encode_vlu(chunk_buf, 1+edp_len)
	binary.Write(chunk_buf, binary.BigEndian, edp_type)
	chunk_buf.Write(edp)

	//Tag
	chunk_buf.Write(tag)

	p := &packet{
		time_stamp: timestamp(),
		mode:       mode_startup,
	}

	p.init()
	p.add_chunk(0x30, chunk_buf.Bytes())

	go self.send_packet(addr, p.pack(0, default_crypto_key))
}

func (self *handshake) recv_ihello(srcAddr *string, edpType uint8, edpData, tag []byte) {
	//fmt.Printf("recv_ihello(edpType:%d edpData:%v tag:%v)\n", edpType, edpData, tag)

	self.send_rhello(*srcAddr, tag)
}

func (self *handshake) recv_fihello(srcAddr *string, edpType uint8, edpData []byte, replyAddress string, tag []byte) {
	//fmt.Printf("recv_fihello(edpType:%d edpData:%v tag:%v)\n", edpType, edpData, tag)

	self.send_rhello(replyAddress, tag)
}

func (self *handshake) send_rhello(dstAddr string, tag []byte) {

	chunk_buf := bytes.NewBuffer(nil)

	//tag echo
	encode_vlu_prefix_bytes(chunk_buf, tag)

	//cookie
	encode_vlu_prefix_bytes(chunk_buf, self.cookie)

	//responder certificate
	chunk_buf.Write(self.certificate)

	p := &packet{
		time_stamp: timestamp(),
		mode:       mode_startup,
	}

	p.init()
	p.add_chunk(0x70, chunk_buf.Bytes())

	go self.send_packet(dstAddr, p.pack(0, default_crypto_key))
}

func (self *handshake) recv_rhello(srcAddr *string, tagEcho, cookie, respCert []byte) {
	//fmt.Printf("recv_rhello(tag:%v)\n", tagEcho)
	//fmt.Printf("recv_rhello(tagEho:%v cookie:%v respCert:%v\n", tagEcho, cookie, respCert)

	//check pending tag.
	req, ok := self.requests[string(tagEcho)]
	if !ok {
		//fmt.Println("invalid tagEcho!")
		return
	}
	delete(self.requests, string(tagEcho))

	var dh_public_number []byte

	//flash client will return dh public number in rhello
	if read_vlu_option(respCert, 0x1D, 0) == 0x02 && len(respCert) > 128 {
		dh_public_number = respCert[len(respCert)-128:]
		//fmt.Println("respCert contain dh public number")

		//adjust respCert
		respCert = respCert[0 : len(respCert)-128]
	}

	//verified peerid.
	farId := gen_peerid_from_cert(respCert)
	if !bytes.Equal(farId, req.target) {
		//fmt.Println("WARNING: respCert is not match to the request peerid!")
	}

	//TODO: check established session by peerid?
	//this happen when peer have multi-ip or we get peer from multi place, and we don't know peerid in advance.

	req.cb(*srcAddr, cookie, dh_public_number)
}

func (self *handshake) recv_redirect(srcAddr *string, tagEcho []byte, redirectDestination []string) {
	//fmt.Printf("recv_redirect(tag:%v %v)\n", tagEcho, redirectDestination)

	//check pending tag.
	req, ok := self.requests[string(tagEcho)]
	if !ok {
		//fmt.Println("invalid tagEcho!")
		return
	}

	for _, dstAddr := range redirectDestination {
		self.send_ihello(dstAddr, req.target, tagEcho)
	}
}

func (self *handshake) recv_iikeying(srcAddr *string, initSid uint32, cookieEcho, initCert, initNonce []byte) {

	//check cookieEcho
	if !bytes.Equal(self.cookie, cookieEcho) {
		//fmt.Println("cookie not match!")
		return
	}

	//TODO: find established session by peerid. this will happen when the rikeying response is lost.

	//NOTE: peerid calc from initCert and respCert is different! so we should call this nearid, only identify this session.
	nearId := gen_peerid_from_cert(initCert)

	if self.create_passive_session == nil {
		//fmt.Println("create_passive_session function not set.")
		panic("create_passive_session function not set.")
		return
	}

	s, err := self.create_passive_session(*srcAddr, nearId)
	if err != nil {
		//fmt.Println(err)
		return
	}

	s.recv_iikeying(srcAddr, initSid, cookieEcho, initCert, initNonce)
}

func (self *handshake) recv_rhello_cookie_change(srcAddr *string, oldCookie, newCookie []byte) {
	panic("should not called!")
}

func (self *handshake) recv_rikeying(srcAddr *string, respSid uint32, respNonce []byte) {
	panic("should not called!")
}
func (self *handshake) recv_ping(srcAddr *string, msg []byte)           { panic("should not called!") }
func (self *handshake) recv_ping_reply(srcAddr *string, msgEcho []byte) { panic("should not called!") }
func (self *handshake) recv_userdata(srcAddr *string, fragmentControl uint8, flowid, sequenceNumber, fsnOffset uint, data, options []byte, abandon, final bool) {
	panic("should not called!")
}
func (self *handshake) recv_range_ack(srcAddr *string, flowid, bufAvail, cumAck uint, recvRanges []Range) {
	panic("should not called!")
}
func (self *handshake) recv_buffer_probe(srcAddr *string, flowid uint) { panic("should not called!") }
func (self *handshake) recv_flow_exception_report(srcAddr *string, flowid, exception uint) {
	panic("should not called!")
}

func (self *handshake) recv_session_close_request() { panic("should not called!") }
func (self *handshake) recv_session_close_ack()     { panic("should not called!") }
