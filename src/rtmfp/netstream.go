package rtmfp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

var net_stream_req_signature = []byte{0x07, 0x00, 0x54, 0x43, 0x04, 0xFA, 0x89, 0x00}
var net_stream_res_signature = []byte{0x05, 0x00, 0x54, 0x43, 0x04, 0x00}

type net_stream struct {
	session  *session
	sendFlow *send_flow
	recvFlow *recv_flow
}

func (self *net_stream) active_open() (flowid uint, err error) {
	self.sendFlow, err = self.session.new_send_flow(0, net_stream_req_signature)

	//the response flow have not yet determine, should wait the remote response.

	return self.sendFlow.flowid, err
}

func (self *net_stream) passive_open(recv_flowid uint) (err error) {
	self.recvFlow, err = self.session.new_recv_flow(recv_flowid)

	//response flow
	self.sendFlow, err = self.session.new_send_flow(recv_flowid, net_stream_res_signature)

	return err
}

func (self *net_stream) close() {

}

func (self *net_stream) attach_flow(flowid uint) bool {
	if self.recvFlow == nil {
		self.recvFlow, _ = self.session.new_recv_flow(flowid)
		return true
	} else {
		//we don't expect this flow
		return false
	}
}

func (self *net_stream) recv() (cmd string, param interface{}, err error) {

	for {
		var buf []byte
		buf, err = self.recvFlow.recv()
		if err != nil {
			return "", nil, err
		}

		cmd, param, err = self.decode_msg(buf)
		if err != nil {
			return
		}

		//do standard reply before return to caller.
		switch cmd {
		case "play":
			self.recv_play(param)
		case "publish":
			self.recv_publish(param)
		}
		return
	}
}

func (self *net_stream) send(cmd string, v interface{}) error {

	//fmt.Printf("send %s()\n", cmd)

	buf, err := self.encode_msg(cmd, v)
	if err != nil {
		return err
	}
	_, err = self.sendFlow.send(buf)

	return err
}

func (self *net_stream) decode_msg(buf []byte) (cmd string, param interface{}, err error) {

	r := bytes.NewBuffer(buf)

	var msg_type uint8
	binary.Read(r, binary.BigEndian, &msg_type)

	//fmt.Printf("msg type:%d\n", msg_type)

	switch msg_type {
	case 0x11: //AMF?
		r.Next(5)
		cmd = read_amf0_type_string(r)
		_ = read_amf0_type_number(r) /*callback*/

		//NOTE: skip the first null
		//when server response, it write "cmd|callback|null|..."
		if r.Bytes()[0] == 0x5 {
			r.Next(1)
		}

		param = decode_amf(r)
	case 0x14: //AMF_WITH_HANDLER
		r.Next(4)
		cmd = read_amf0_type_string(r)
		_ = read_amf0_type_number(r) /*callback*/

		//NOTE: skip the first null
		//when server response, it write "cmd|callback|null|..."
		if r.Bytes()[0] == 0x5 {
			r.Next(1)
		}

		param = decode_amf(r)
	case 0x0F: //AMF
		r.Next(5)
		cmd = read_amf0_type_string(r)
		param = decode_amf(r)
	default:
		fmt.Printf("###############unknown msg type:%d####################\n", msg_type)
		//panic("unknown msg type")
	}

	//fmt.Printf("#cmd:%s\n", cmd)

	return
}

func (self *net_stream) encode_msg(cmd string, v interface{}) (buf []byte, err error) {

	w := bytes.NewBuffer(nil)

	//AMF_WITH_HANDLER

	binary.Write(w, binary.BigEndian, uint8(0x14))
	binary.Write(w, binary.BigEndian, uint32(0))

	write_amf0_type_string(w, cmd)
	write_amf0_type_number(w, 0) //FIXME: ?
	write_amf0_type_null(w)      //FIXME: should we write this?

	encode_amf(w, v)

	return w.Bytes(), nil
}

func (self *net_stream) play(name string) {
	//fmt.Printf("net_stram::play(%s)\n", name)

	self.send("play", name)
}

func (self *net_stream) recv_play(param interface{}) {

	name := param.(string)
	//fmt.Printf("recv_play: %s\n", name)

	res := make(map[string]interface{})
	res["level"] = "status"
	res["code"] = "NetStream.Play.Reset"
	res["description"] = name + " is reset!"

	self.send("onStatus", res)

	res["code"] = "NetStream.Play.Start"
	res["description"] = name + " is playing!"

	self.send("onStatus", res)
}

func (self *net_stream) publish(name string) {
	//fmt.Printf("net_stram::publish(%s)\n", name)

	self.send("publish", name)
}

func (self *net_stream) recv_publish(param interface{}) {

	res := make(map[string]interface{})
	res["level"] = "status"
	res["code"] = "NetStream.Publish.Start"
	res["description"] = param.(string) + " is now published!"

	self.send("onStatus", res)
}

func (self *net_stream) dump_state(w io.Writer) {

	if self.session != nil {
		self.session.dump_state(w)
		fmt.Fprintln(w)
	}

	if self.sendFlow != nil {
		self.sendFlow.dump_state(w)
		fmt.Fprintln(w)
	}

	if self.recvFlow != nil {
		self.recvFlow.dump_state(w)
		fmt.Fprintln(w)
	}
}
