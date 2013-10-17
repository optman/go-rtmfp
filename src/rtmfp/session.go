package rtmfp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"
)

var dh1024p = "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF"

var mode_startup = uint8(3)
var mode_initiator = uint8(1)
var mode_responder = uint8(2)

var iikeying_retry_count = 5 //not exceed 64
var iikeying_timeout = 1 * time.Second

type session struct {
	in  chan *network_packet
	out chan *network_packet

	sessionid, other_sessionid uint32
	dh_private, dh_public      *big.Int
	other_dh_public            []byte
	nonce                      []byte
	mode                       uint8
	dkey, ekey                 []byte

	other_addr  string
	last_flowid uint

	send_flows map[uint]*send_flow
	recv_flows map[uint]*recv_flow

	create_recv_flow   func(options []byte, flowid uint) (*recv_flow, error)
	recv_close_request func()

	active_open_chan chan bool

	mobile_tx_ts time.Time

	//RTT related
	ts_rx      uint16        //last timestamp received from far end
	ts_echo_tx uint16        //last timestamp echo sent to far end
	ts_rx_time time.Time     //the time at which TS_RX was first observed to be different than its previous value
	ts_tx      uint16        //last timestamp sent to far end
	ts_echo_rx uint16        //last timestamp echo received from far end
	srtt       time.Duration //smooth rtt
	rttvar     time.Duration //rtt variant
	mrto       time.Duration //measure retransmit timeout
	erto       time.Duration //effective restransmit timeout

	//stastics
	c_ack_rx       int
	c_ack_tx       int
	c_user_data_rx int
	c_user_data_tx int
	c_packet_rx    int
	c_packet_tx    int
}

func (self *session) init_dh() {

	//g,p
	var p big.Int
	p.SetString(dh1024p, 16)
	g := big.NewInt(2)

	//x
	dh_private := make([]byte, 128)
	rand.Read(dh_private)
	self.dh_private = new(big.Int)
	self.dh_private.SetString(hex.EncodeToString(dh_private), 16)

	//y = g^x mod p
	self.dh_public = g.Exp(g, self.dh_private, &p)

	//generate nonce
	self.nonce = make([]byte, 139) //11 + 128
	copy(self.nonce, []byte("\x03\x1A\x00\x00\x02\x1E\x00\x81\x02\x0D\x02"))
	pad_len := 128 - len(self.dh_public.Bytes())
	copy(self.nonce[11+pad_len:], self.dh_public.Bytes())
}

func (self *session) end_dh(other_dh_pub []byte) *big.Int {

	//p
	var p big.Int
	p.SetString(dh1024p, 16)

	y := new(big.Int)
	y.SetString(hex.EncodeToString(other_dh_pub), 16)

	//share_secret = y^x mode p
	share_secret := y.Exp(y, self.dh_private, &p)

	//fmt.Printf("share_secret:%v\n", share_secret)

	return share_secret
}

func (self *session) generate_aes_keys(other_dh_pub, myNonce, otherNonce []byte) {

	self.dkey, self.ekey = GenCryptoKeys(self.end_dh(other_dh_pub), myNonce, otherNonce)
}

func (self *session) init() {
	self.send_flows = make(map[uint]*send_flow)
	self.recv_flows = make(map[uint]*recv_flow)
	self.mode = mode_startup
	self.init_dh()

	//rtt related
	self.mrto = 250 * time.Microsecond
	self.erto = 3 * time.Second

	go self.dispatch()
}

func (self *session) handshaked() bool {
	return self.ekey != nil
}

func (self *session) active_open(dstAddr string, cookie, other_dh_public []byte) error {
	self.init()

	self.other_dh_public = other_dh_public

	self.active_open_chan = make(chan bool, 1)
forloop:
	for i := 0; i < iikeying_retry_count; i++ {

		self.send_iikeying(dstAddr, cookie)

		select {
		case <-self.active_open_chan:
			break forloop
		case <-time.After((1 << uint(i)) * iikeying_timeout):
		}
	}

	self.active_open_chan = nil

	if self.handshaked() {
		return nil
	} else {
		return errors.New("active open fail!")
	}
}

func (self *session) passive_open() {
	self.init()
}

func (self *session) close() {

	self.send_session_close_request()

	for _, flow := range self.recv_flows {
		flow.close()
	}

	for _, flow := range self.send_flows {
		flow.close()
	}
}

func (self *session) dispatch() {

	for {
		packet, ok := <-self.in
		if !ok {
			break
		}
		self.recv_packet(packet)
	}
}

//should not be called
func (self *session) recv_ihello(srcAddr *string, edpType uint8, edpData, tag []byte) {}
func (self *session) recv_fihello(srcAddr *string, edpType uint8, edpData []byte, replyAddress string, tag []byte) {
}

func (self *session) recv_rhello(srcAddr *string, tagEcho, cookie, respCert []byte) {}
func (self *session) recv_redirect(srcAddr *string, tagEcho []byte, redirectDestination []string) {
}

func (self *session) recv_rhello_cookie_change(srcAddr *string, oldCookie, newCookie []byte) {
	//naive implementation
	self.send_iikeying(*srcAddr, newCookie)
}

func (self *session) send_iikeying(dstAddr string, cookieEcho []byte) {

	//fmt.Printf("send_iikeying(%s)\n", dstAddr)

	chunk_buf := bytes.NewBuffer(nil)

	//initiator sessionid
	binary.Write(chunk_buf, binary.BigEndian, self.sessionid)

	//cookie echo
	encode_vlu_prefix_bytes(chunk_buf, cookieEcho)

	//initiator certificate
	encode_vlu(chunk_buf, 132) //4 + 128
	chunk_buf.Write([]byte("\x81\x02\x1D\x02"))

	pad_len := 128 - len(self.dh_public.Bytes())
	for i := 0; i < pad_len; i++ {
		chunk_buf.WriteByte(0x0)
	}

	chunk_buf.Write(self.dh_public.Bytes())

	//session key initiator component
	encode_vlu_prefix_bytes(chunk_buf, self.nonce)

	chunk_buf.WriteByte(0x58)

	self.send_chunk(dstAddr, 0x38, chunk_buf.Bytes())
}

func (self *session) recv_iikeying(srcAddr *string, initSid uint32, cookieEcho, initCert, initNonce []byte) {
	//fmt.Printf("recv_iikeying(%s initSid:%d)\n", *srcAddr, initSid)

	/* it will be checked at outside code, as handshake object.

	if !bytes.Equal(cookieEcho, self.cookie) {
		fmt.Println("cookie not match!")
		return
	}*/

	//CERT = OPTION(x1D, \x02 + DH)
	other_dh_pub := read_option(initCert, 0x1D)[1:]

	self.generate_aes_keys(other_dh_pub, self.nonce, initNonce)

	//session established for responder
	self.mode = mode_responder
	self.other_sessionid = initSid
	self.other_addr = *srcAddr

	self.send_rikeying(*srcAddr)
}

func (self *session) send_rikeying(dstAddr string) {
	//fmt.Printf("send_rikeying(%s respSid:%d)\n", dstAddr, self.other_sessionid)

	chunk_buf := bytes.NewBuffer(nil)

	//responder sessionid
	binary.Write(chunk_buf, binary.BigEndian, self.sessionid)

	//session key responder component
	encode_vlu_prefix_bytes(chunk_buf, self.nonce)

	chunk_buf.WriteByte(0x58)

	self.send_chunk(dstAddr, 0x78, chunk_buf.Bytes())
}

func (self *session) recv_rikeying(srcAddr *string, respSid uint32, respNonce []byte) {
	//fmt.Printf("recv_rikeying(respSid:%d)\n", respSid)

	if self.handshaked() {
		//fmt.Println("drop duplicated rikeying.")
		return
	}

	//CERT = OPTION(x0D, \x02 + DH)
	option_0d := read_option(respNonce, 0x0D)
	if self.other_dh_public == nil && len(option_0d) > 1 {
		self.other_dh_public = option_0d[1:]
	}

	self.generate_aes_keys(self.other_dh_public, self.nonce, respNonce)

	//session established for both side!
	self.mode = mode_initiator
	self.other_sessionid = respSid
	self.other_addr = *srcAddr

	if self.active_open_chan != nil {
		self.active_open_chan <- true
	}
}

func (self *session) send_ping(dstAddr string) {
	chunk_buf := bytes.NewBuffer(nil)
	chunk_buf.Write([]byte("hello"))

	self.send_chunk(dstAddr, 0x01, chunk_buf.Bytes())
}

func (self *session) recv_ping(srcAddr *string, msg []byte) {
	//fmt.Printf("recv_ping(%v)\n", msg)

	self.send_ping_reply(*srcAddr, msg)
}

func (self *session) send_ping_reply(dstAddr string, msgEcho []byte) {
	chunk_buf := bytes.NewBuffer(nil)
	chunk_buf.Write(msgEcho)

	self.send_chunk(dstAddr, 0x41, chunk_buf.Bytes())
}

func (self *session) recv_ping_reply(srcAddr *string, msgEcho []byte) {
	//fmt.Printf("recv_ping_reply(%v)\n", msgEcho)

	//address change confirm
	if *srcAddr != self.other_addr && time.Since(self.mobile_tx_ts) < 120*time.Second {
		fmt.Printf("new remote address confirmed! %v -> %v\n", self.other_addr, *srcAddr)
		self.other_addr = *srcAddr
	}
}

func (self *session) send_userdata(fragmentControl uint8, flowid, sequnceNumber, fsnOffset uint, data, options []byte, abandon, final bool) {
	self.c_user_data_tx++

	//fmt.Printf("send_userdata(flowid: %d sequnceNumber: %d)\n", flowid, sequnceNumber)

	chunk_buf := bytes.NewBuffer(nil)

	var flags uint8

	flags = fragmentControl << 4

	if options != nil {
		flags |= 0x80
	}

	if abandon {
		flags |= 0x02
	}

	if final {
		flags |= 0x01
	}

	binary.Write(chunk_buf, binary.BigEndian, flags)
	encode_vlu(chunk_buf, flowid)
	encode_vlu(chunk_buf, sequnceNumber)
	encode_vlu(chunk_buf, fsnOffset)

	if options != nil {
		chunk_buf.Write(options)
	}

	chunk_buf.Write(data)

	self.send_chunk(self.other_addr, 0x10, chunk_buf.Bytes())
}

func (self *session) recv_userdata(srcAddr *string, fragmentControl uint8, flowid, sequenceNumber, fsnOffset uint, data, options []byte, abandon, final bool) {
	self.c_user_data_rx++

	//fmt.Printf("recv_userdata(%d-%d-%d)\n", flowid, sequenceNumber, fsnOffset)

	//TODO: detect source address changed.

	flow, ok := self.recv_flows[flowid]
	if !ok {

		//notify owner a new flow is coming, it will create a recv_flow for it.
		if self.create_recv_flow != nil {

			flow, _ = self.create_recv_flow(options, flowid)

		} else {

			//auto accept new incoming flow.
			//flow, _ = self.new_recv_flow(flowid)
		}
	}

	if flow != nil {
		flow.on_userdata(fragmentControl, sequenceNumber, fsnOffset, data, options, abandon, final)
	} else {
		self.send_flow_exception_report(flowid, 0)
		panic("unknown flow.")
	}
}

func (self *session) send_range_ack(flowid, bufAvail, cumAck uint, recvRanges []Range) {
	self.c_ack_tx++

	chunk_buf := bytes.NewBuffer(nil)

	encode_vlu(chunk_buf, flowid)
	encode_vlu(chunk_buf, bufAvail/1024)
	encode_vlu(chunk_buf, cumAck)

	ackCursor := cumAck + 1

	for _, rr := range recvRanges {
		encode_vlu(chunk_buf, rr.Pos-ackCursor-1)
		encode_vlu(chunk_buf, rr.Len-1)

		ackCursor = rr.End()
	}

	self.send_chunk(self.other_addr, 0x51, chunk_buf.Bytes())
}

func (self *session) recv_range_ack(srcAddr *string, flowid, bufAvail, cumAck uint, recvRanges []Range) {
	self.c_ack_rx++

	//fmt.Printf("recv_ack(%d-%d-%d)\n", flowid, bufAvail, cumAck)

	flow, ok := self.send_flows[flowid]
	if !ok {
		//fmt.Printf("unknown flowid:%d\n", flowid)
		return
	}

	flow.on_range_ack(bufAvail, cumAck, recvRanges)
}

func (self *session) send_buffer_probe(flowid uint) {
	chunk_buf := bytes.NewBuffer(nil)

	encode_vlu(chunk_buf, flowid)

	self.send_chunk(self.other_addr, 0x18, chunk_buf.Bytes())
}

func (self *session) recv_buffer_probe(srcAddr *string, flowid uint) {
	flow, ok := self.recv_flows[flowid]
	if !ok {
		//fmt.Printf("unknown flowid:%d\n", flowid)
		return
	}

	flow.on_buffer_probe()
}

func (self *session) send_flow_exception_report(flowid, exception uint) {
	chunk_buf := bytes.NewBuffer(nil)

	encode_vlu(chunk_buf, flowid)
	encode_vlu(chunk_buf, exception)

	self.send_chunk(self.other_addr, 0x5e, chunk_buf.Bytes())
}

func (self *session) recv_flow_exception_report(srcAddr *string, flowid, exception uint) {
	flow, ok := self.send_flows[flowid]
	if !ok {
		//fmt.Printf("unknown flowid:%d\n", flowid)
		return
	}

	flow.on_flow_exception_report(exception)
}

func (self *session) send_session_close_request() {
	self.send_chunk(self.other_addr, 0x0c, nil)
}

func (self *session) recv_session_close_request() {
	fmt.Println("recv_session_close_request")

	if self.recv_close_request != nil {
		self.recv_close_request()
	}

	self.send_session_close_ack()
}

func (self *session) send_session_close_ack() {
	self.send_chunk(self.other_addr, 0x4c, nil)
}

func (self *session) recv_session_close_ack() {
	//do actual close session
}

func (self *session) send_chunk(dstAddr string, chunk_type uint8, chunk_data []byte) {
	p := &packet{
		mode: self.mode,
	}

	//include timestamp only when changed from last timestamp()
	cur_ts := timestamp()
	if self.ts_tx != cur_ts {
		self.ts_tx = cur_ts
		p.time_stamp = cur_ts
	}

	//include timestam echo within 128s after last recv timestamp.
	ts_rx_elapsed := time.Since(self.ts_rx_time)
	if ts_rx_elapsed > 128*time.Second {
		self.ts_rx = 0
		self.ts_rx_time = time.Time{}
	} else {

		ts_rx_elapsed_ticks := uint16(ts_rx_elapsed / (4 * time.Millisecond))

		ts_echo := (self.ts_rx + ts_rx_elapsed_ticks) /*% 65536*/

		if ts_echo != self.ts_echo_tx {
			self.ts_echo_tx = ts_echo
			p.time_stamp_echo = ts_echo
		}

	}

	//RIKeying is special, other_sessionid != 0 and use default crypt key, shoule be in startup mode
	crypt_key := self.ekey
	if chunk_type == 0x78 { //RIKeying is special
		p.mode = mode_startup
		crypt_key = nil
	}

	p.init()
	p.add_chunk(chunk_type, chunk_data)

	self.send_packet(dstAddr, p.pack(self.other_sessionid, crypt_key))
}

func (self *session) send_packet(dstAddr string, data []byte) {
	self.c_packet_tx++
	self.out <- &network_packet{addr: dstAddr, data: data}
}

func (self *session) recv_packet(p *network_packet) {
	self.c_packet_rx++
	decode_packet(&p.addr, p.data, self.dkey, self, self)

	//destaddr changed?
	if p.addr != self.other_addr && time.Since(self.mobile_tx_ts) > 1*time.Second {
		fmt.Printf("detect remote address changed. new address: %v\n", p.addr)
		self.mobile_tx_ts = time.Now()
		self.send_ping(p.addr)
	}
}

func (self *session) recv_packet_info(srcAddr *string, sid uint32, timeCritical, timeCriticalReverse bool,
	mode uint8, ts, timestampEcho uint16) {

	if ts != 0 && ts != self.ts_rx {
		self.ts_rx = ts
		self.ts_rx_time = time.Now()
	}

	if timestampEcho != 0 && timestampEcho != self.ts_echo_rx {
		self.ts_echo_rx = timestampEcho

		rtt_ticks := (timestamp() - timestampEcho) /*% 65536*/
		if rtt_ticks <= 32767 {
			rtt := time.Duration(rtt_ticks) * 4 * time.Millisecond

			if self.srtt != 0 {
				var rtt_delta time.Duration
				if self.srtt > rtt {
					rtt_delta = self.srtt - rtt
				} else {
					rtt_delta = rtt - self.srtt
				}

				self.rttvar = (3*self.rttvar + rtt_delta) / 4
				self.srtt = (7*self.srtt + rtt) / 8
			} else {
				self.srtt = rtt
				self.rttvar = rtt / 2
			}

			self.mrto = self.srtt + 4*self.rttvar + 200*time.Millisecond

			self.erto = max_duration(self.mrto, 250*time.Millisecond)
		}
	}

	//fmt.Printf("MRTO: %v ERTO: %v SRTT: %v\n", self.mrto, self.erto, self.srtt)
}

func (self *session) new_flowid() uint {
	self.last_flowid++
	return self.last_flowid
}

func (self *session) new_send_flow(rel_flowid uint, signature []byte) (*send_flow, error) {

	flow := &send_flow{
		flowid:     self.new_flowid(),
		rel_flowid: rel_flowid,
		session:    self,
		signature:  signature,
	}
	flow.open()

	self.send_flows[flow.flowid] = flow

	return flow, nil
}

func (self *session) new_recv_flow(recv_flowid uint) (*recv_flow, error) {

	flow := &recv_flow{
		flowid:  recv_flowid,
		session: self,
	}
	flow.open()

	self.recv_flows[flow.flowid] = flow

	return flow, nil
}

func (self *session) dump_state(w io.Writer) {
	fmt.Fprintf(w, "[SESSION]\n")
	fmt.Fprintf(w, "packet_tx: %d\tuserdata_tx: %d\tack_tx: %d\n", self.c_packet_tx, self.c_user_data_tx, self.c_ack_tx)
	fmt.Fprintf(w, "packet_rx: %d\tuserdata_rx: %d\tack_rx: %d\n", self.c_packet_rx, self.c_user_data_rx, self.c_ack_rx)
	fmt.Fprintf(w, "mrto: %d\terto: %d\tsrtt: %d\n", self.mrto/time.Millisecond, self.erto/time.Millisecond, self.srtt/time.Millisecond)
}
