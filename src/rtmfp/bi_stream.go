//bi_stream is stand for bidirectional stream. which can send and recv data in two ways,
//underneath we a single net_stream object to send and recv data, while under standard usage,
//netstream can either recv or send data but not both.
//by ignore the publish/play model, we enable net_stream send/recv in both direction

//to interoperate with adobe flash client, we still implement the publish/play command, but no real meaning.

//to create a bi_stream

//active open:
//first create a netstream, and play a remote flow, this  will enable you receive data.
//receive play.start status notify message, bi_stream establish ok.
//use send() method to send payload data.

//passive open:
//receive play cmd, check or ignore what ever stream name, return play.start status message.  bi_stream establish ok.
//use send() method to send payload data.

//send(handler, param)
//handler is fixed as "__data" or what ever you like.
//param should be AMF ByteArray type, this is our actual payload.

package rtmfp

import (
	"errors"
	"fmt"
	"io"
	"time"
)

var bi_stream_handler = "__data"

type bi_stream struct {
	name string
	ns   *net_stream

	received_msgs chan []byte
	closed        bool
}

func (self *bi_stream) active_open_dispatch(play_start_event chan bool) {
	for {
		cmd, param, err := self.ns.recv()
		if err != nil {
			break
		}

		if cmd == "onStatus" {
			obj := param.(map[string]interface{})
			//fmt.Printf("onStatus:%s\n", obj["code"])
			if obj["code"] == "NetStream.Play.Start" {
				play_start_event <- true
			}
		} else if cmd == bi_stream_handler {
			self.received_msgs <- param.([]byte)
		} else {
			fmt.Printf("unknown cmd:%s\n", cmd)
		}

	}
}

func (self *bi_stream) passive_open_dispatch(play_event chan bool) {
	for !self.closed {
		cmd, param, err := self.ns.recv()
		if err != nil || self.closed {
			break
		}

		if cmd == "play" {
			stream_name := param.(string)
			//fmt.Printf("play(%s)\n", stream_name)

			if len(self.name) == 0 || stream_name == self.name {
				play_event <- true
			} else {
				play_event <- false
				panic("wrong stream name! ")
			}
		} else if cmd == "closeStream" {
			self.close()
		} else if cmd == bi_stream_handler {
			self.received_msgs <- param.([]byte)
		} else {
			fmt.Printf("unknown cmd:%s\n", cmd)
		}
	}
}

func (self *bi_stream) active_open(session *session, dstStream string) (err error) {

	if session == nil {
		panic("session not empty!")
	}

	self.ns = &net_stream{
		session: session,
	}

	self.received_msgs = make(chan []byte, 1000)

	var active_flowid uint
	active_flowid, err = self.ns.active_open()
	if err != nil {
		return err
	}

	play_start_event := make(chan bool, 1)

	session.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

		rel_flowid := read_vlu_option(options, 0xa, 0)

		if rel_flowid == active_flowid { //associate reply flow.
			self.ns.attach_flow(flowid)
			go self.active_open_dispatch(play_start_event)

			return self.ns.recvFlow, nil

		} else {
			fmt.Println("not expect this flow!")
			//panic("not expect this flow!")
			return nil, errors.New("not expect this flow!")
		}
	}

	self.ns.play(dstStream)

	//received play.start?
	select {
	case <-play_start_event:
	case <-time.After(3 * time.Second):
		return errors.New("not recv play.start in 3s.")
	}

	//fmt.Println("active open ok!")

	return nil
}

func (self *bi_stream) passive_open(session *session, dstStream string) error {

	if session == nil {
		panic("session empty!")
	}

	self.received_msgs = make(chan []byte, 1000)

	play_event := make(chan bool, 1)

	session.create_recv_flow = func(options []byte, flowid uint) (*recv_flow, error) {

		if self.ns == nil {

			self.ns = &net_stream{
				session: session,
			}

			self.ns.passive_open(flowid)
			go self.passive_open_dispatch(play_event)

			return self.ns.recvFlow, nil

		} else {
			panic("not expect this flow!")
			return nil, errors.New("not expect this flow!")
		}
	}

	session.recv_close_request = func() {
		self.close()
	}

	//received play?
	//don't wait the play cmd, just return ok.
	/*
		select {
		case <-play_event:
		case <-time.After(3 * time.Second):
			return errors.New("not recv play in 3s.")
		}*/

	//fmt.Println("passive open ok!")

	return nil
}

func (self *bi_stream) close() {
	if !self.closed {

		self.closed = true
		self.ns.close()

		close(self.received_msgs)

		//close the session because it is the only user.
		self.ns.session.close()
	}
}

func (self *bi_stream) send(data []byte) error {
	return self.ns.send(bi_stream_handler, data)
}

func (self *bi_stream) recv() ([]byte, error) {

	data, ok := <-self.received_msgs

	if ok {
		return data, nil
	} else {
		return nil, errors.New("received msgs channel closed!")
	}
}

func (self *bi_stream) dump_state(w io.Writer) {
	if self.ns != nil {
		self.ns.dump_state(w)
	}
}
