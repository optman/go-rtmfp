package rtmfp

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"
)

var smss = uint(1460) //Sender Maximum Segment Size(SMSS)
var init_recv_wnd = smss * 30
var init_cong_wnd = smss * 3
var timeout_cong_wnd = smss
var init_ssthresh = ^uint(0) //max uint

var fastgrow_allowed = false
var is_time_critical = false

var max_recv_buf_size = uint(1 * 1024 * 1024)
var max_data_chunk_resend_count = int(10)

var bufprob_ticker_duration = 100 * time.Millisecond

var fc_whole = uint8(0)
var fc_begin = uint8(1)
var fc_middle = uint8(3)
var fc_end = uint8(2)

type send_flow struct {
	flowid, rel_flowid uint
	session            *session

	last_seqnum uint

	send_queue              *list.List
	inflight_bytes          uint
	acked_bytes_accumulator uint

	recv_wnd           uint
	cong_wnd           uint
	ssthresh           uint
	ack_ranges         RangeQueue
	data_packets_count int //user data sent since last received ack.

	bufprob_ticker *time.Ticker
	rtx_alarm      *time.Timer

	current_tsn int

	signature []byte

	closed bool

	c_loss int
}

type recv_flow struct {
	flowid  uint
	session *session

	ordered_recv_buf   *list.List
	unordered_recv_buf *data_chunk_heap

	ordered_recved_bytes, unordered_recv_buf_bytes uint
	recv_ranges                                    RangeQueue
	rx_data_packets                                int //recv user data count since last sent ack
	prev_rwnd                                      int //the most recent receive window advertisement sent in an acknowledgement, in 1024-byte blocks

	last_ordered_seqnum uint

	recv_buf_mutex sync.Mutex
	recv_cond      *sync.Cond

	delack_alarm *time.Timer

	closed bool
}

func (self *send_flow) open() {
	self.send_queue = list.New()
	self.recv_wnd = init_recv_wnd
	self.cong_wnd = init_cong_wnd
	self.ssthresh = init_ssthresh
}

func (self *send_flow) close() {
	self.closed = true

	if self.rtx_alarm != nil {
		self.rtx_alarm.Stop()
		self.rtx_alarm = nil
	}

	if self.bufprob_ticker != nil {
		self.bufprob_ticker.Stop()
		self.bufprob_ticker = nil
	}
}

func (self *send_flow) next_seqnumber() uint {
	self.last_seqnum++
	return self.last_seqnum
}

//data parameter is view as a message, which will delieve to the receiver as a whole.
func (self *send_flow) send(data []byte) (uint, error) {

	if self.closed {
		return 0, errors.New("flow closed!")
	}

	//put data into standby queue.
	if uint(len(data)) <= smss {
		chunk := &data_chunk{
			fragCtrl: fc_whole,
			seqNum:   self.next_seqnumber(),
			data:     data,
		}

		self.send_queue.PushBack(chunk)
	} else {
		//split message into multi fragment.

		for i := uint(0); i < uint(len(data)); {

			chunk := &data_chunk{seqNum: self.next_seqnumber()}

			chunk_length := smss
			if (i + chunk_length) >= uint(len(data)) {
				chunk_length = uint(len(data)) - i
				chunk.fragCtrl = fc_end
			} else if i == 0 {
				chunk.fragCtrl = fc_begin
			} else {
				chunk.fragCtrl = fc_middle
			}

			chunk.data = data[i : i+chunk_length]

			self.send_queue.PushBack(chunk)

			i += chunk_length
		}
	}

	self.try_send()

	return uint(len(data)), nil
}

func (self *send_flow) send_buget() bool {
	return self.inflight_bytes < min_uint(self.recv_wnd, self.cong_wnd)
}

func (self *send_flow) burst_avoid() bool {
	return self.data_packets_count < 6
}

func (self *send_flow) try_send() {

	for i := self.send_queue.Front(); i != nil && self.send_buget() && self.burst_avoid(); i = i.Next() {

		chunk := i.Value.(*data_chunk)
		if chunk.in_flight {
			continue
		}

		self.send_userdata(chunk)
	}
}

func (self *send_flow) on_rtx_alarm() {

	any_loss := false

	for i := self.send_queue.Front(); i != nil; i = i.Next() {

		chunk := i.Value.(*data_chunk)
		if !chunk.in_flight {
			continue
		}

		if chunk.send_count > max_data_chunk_resend_count {

			//fmt.Println(self.ack_ranges.String())

			fmt.Printf("seq_num(%d) resend(%d) exceed max resend count(%d)! close the flow.\n",
				chunk.seqNum, chunk.send_count, max_data_chunk_resend_count)

			self.close()
			return
		}

		any_loss = true
		self.chunk_loss(chunk)
	}

	if any_loss {
		self.cong_wnd = timeout_cong_wnd

		//erto backoff?
		erto_backoff := time.Duration(int64(float64(self.session.erto) * 1.4142))
		erto_capped := min_duration(erto_backoff, 10*time.Second)
		self.session.erto = max_duration(erto_capped, self.session.mrto)
	} else {
		self.cong_wnd = init_cong_wnd
	}

	self.ssthresh = max_uint(self.ssthresh, self.cong_wnd*3/4)
	self.acked_bytes_accumulator = 0
	self.data_packets_count = 0

	self.try_send()
}

func (self *send_flow) chunk_loss(chunk *data_chunk) {
	chunk.in_flight = false
	self.inflight_bytes -= uint(len(chunk.data))
	self.c_loss++
}

func (self *send_flow) next_tsn() int {
	self.current_tsn++
	return self.current_tsn
}

func (self *send_flow) send_userdata(chunk *data_chunk) {

	self.data_packets_count++
	self.inflight_bytes += uint(len(chunk.data))

	chunk.send_count++
	chunk.nak_count = 0
	chunk.in_flight = true
	chunk.tsn = self.next_tsn()

	var options []byte
	if chunk.seqNum == 1 {

		options_buf := bytes.NewBuffer(nil)
		if self.signature != nil {
			options_buf.Write(self.signature)
		}

		if self.rel_flowid != 0 {

			encode_vlu(options_buf, 1+get_vlu_size(self.rel_flowid))
			options_buf.WriteByte(0x0a)
			encode_vlu(options_buf, self.rel_flowid)
		}

		if options_buf.Len() > 0 {
			options_buf.WriteByte(0)
			options = options_buf.Bytes()
		}
	}

	self.session.send_userdata(chunk.fragCtrl,
		self.flowid,
		chunk.seqNum,
		chunk.seqNum-self.send_queue.Front().Value.(*data_chunk).seqNum, //fsnOffset
		chunk.data,
		options,
		false,
		false)

	if self.rtx_alarm == nil {
		self.rtx_alarm = time.AfterFunc(self.session.erto, func() { self.on_rtx_alarm() })
	} else {
		self.rtx_alarm.Reset(self.session.erto)
	}
}

func (self *send_flow) on_range_ack(bufAvail, cumAck uint, recvRanges []Range) {

	self.data_packets_count = 0

	pre_ack_outstanding := self.inflight_bytes

	//update recvRange
	self.ack_ranges.AddRange(MakeRange(0, cumAck+1))
	self.ack_ranges.AddRangeQueue(RangeQueueFromArray(recvRanges))

	acked_bytes := uint(0)
	valid := false

	max_tsn := 0

	//remove all acked outstanding.
	for i := self.send_queue.Front(); i != nil; {

		chunk := i.Value.(*data_chunk)

		if self.ack_ranges.Contain(chunk.seqNum) {
			n := i
			i = i.Next()
			self.send_queue.Remove(n)
			if chunk.in_flight {
				self.inflight_bytes -= uint(len(chunk.data))
				acked_bytes += uint(len(chunk.data))
			}

			if chunk.tsn > max_tsn {
				max_tsn = chunk.tsn
			}

			valid = true
		} else {
			i = i.Next()
		}
	}

	//recovery from full buffer.
	if bufAvail > 0 && self.bufprob_ticker != nil {
		self.bufprob_ticker.Stop()
		self.bufprob_ticker = nil
		valid = true
	}

	//this is a delay ack
	if !valid {
		return
	}

	//calc negative ack
	any_nak := false
	any_loss := false
	for i := self.send_queue.Front(); i != nil; i = i.Next() {
		chunk := i.Value.(*data_chunk)
		if chunk.in_flight && chunk.tsn < max_tsn {
			chunk.nak_count++
			any_nak = true
			if chunk.nak_count >= 3 {
				any_loss = true
				self.chunk_loss(chunk)
			}
		}
	}

	//perpare buffer probe
	if bufAvail == 0 && self.bufprob_ticker == nil {

		self.bufprob_ticker = time.NewTicker(bufprob_ticker_duration)
		go func() {

			for {
				_, ok := <-self.bufprob_ticker.C
				if !ok {
					break
				}
				self.session.send_buffer_probe(self.flowid)
			}
		}()
	}

	self.recv_wnd = bufAvail

	self.update_congestion_wnd(any_loss, true, any_nak, acked_bytes, pre_ack_outstanding)

	if self.rtx_alarm != nil {
		self.rtx_alarm.Reset(self.session.erto)
	}

	self.try_send()
}

func (self *send_flow) on_flow_exception_report(exception uint) {
	//FIXME: close flow
	//panic("flow exception.")

	//fmt.Println("######### flow exception ###########")
}

func (self *send_flow) update_congestion_wnd(any_loss, any_ack, any_nak bool, acked_bytes, pre_ack_outstanding uint) {

	if any_loss == true {
		if is_time_critical == true ||
			(pre_ack_outstanding > 67200 && fastgrow_allowed == true) {
			self.ssthresh = max_uint(pre_ack_outstanding*7/8, init_cong_wnd)
		} else {
			self.ssthresh = max_uint(pre_ack_outstanding*1/2, init_cong_wnd)
		}

		self.cong_wnd = self.ssthresh
		self.acked_bytes_accumulator = 0
	} else if any_ack == true && any_nak == false && pre_ack_outstanding >= self.cong_wnd {
		var increase, aithresh uint

		if fastgrow_allowed == true {
			if self.cong_wnd < self.ssthresh {
				increase = acked_bytes
			} else {
				self.acked_bytes_accumulator += acked_bytes
				aithresh = min_uint(max_uint(self.cong_wnd/16, 64), 4800)
				for self.acked_bytes_accumulator >= aithresh {
					self.acked_bytes_accumulator -= aithresh
					increase += 48
				}
			}
		} else {
			if self.cong_wnd < self.ssthresh && is_time_critical == true {
				increase = uint(math.Ceil(float64(acked_bytes) / 4.0))
			} else {
				var aithresh_cap uint
				if is_time_critical {
					aithresh_cap = uint(2400)
				} else {
					aithresh_cap = uint(4800)
				}
				self.acked_bytes_accumulator += acked_bytes

				aithresh = min_uint(max_uint(self.cong_wnd/16, 64), aithresh_cap)
				for self.acked_bytes_accumulator >= aithresh {
					self.acked_bytes_accumulator -= aithresh
					increase += 24
				}
			}

		}

		self.cong_wnd = max_uint(self.cong_wnd+min_uint(increase, smss), init_cong_wnd)
	}

	//a simple implementation
	/*
		if any_ack {
			self.cong_wnd += uint(acked_bytes / 4)
		}

		if any_loss || any_nak {
			self.cong_wnd = self.cong_wnd * 7 / 8
			//self.cong_wnd = max_uint(self.cong_wnd, init_cong_wnd)
		}
	*/
}

func (self *send_flow) dump_state(w io.Writer) {
	fmt.Fprintln(w, "[SEND_FLOW]")
	fmt.Fprintf(w, "inflight: %d\t\nrecvwnd: %v\tcongwnd: %v\t\nlosss: %d\tlossrate: %d%%\n ",
		self.inflight_bytes, self.recv_wnd, self.cong_wnd, self.c_loss, self.c_loss*100/self.session.c_user_data_tx)
}

func (self *recv_flow) open() {
	self.ordered_recv_buf = list.New()
	self.unordered_recv_buf = create_data_chunk_heap()
	self.recv_cond = sync.NewCond(&self.recv_buf_mutex)
}

func (self *recv_flow) close() {
	self.closed = true

	self.recv_cond.Signal()
}

func (self *recv_flow) on_buffer_probe() {
	self.send_ack()
}

func (self *recv_flow) available_buffers() uint {
	total_occupied := self.ordered_recved_bytes + self.unordered_recv_buf_bytes
	if total_occupied >= max_recv_buf_size {
		return 0
	} else {
		return max_recv_buf_size - total_occupied
	}
}

func (self *recv_flow) recv() ([]byte, error) {

	var msg []byte
	for !self.closed {

		self.recv_cond.L.Lock()
		msg = self.read_message()
		if msg == nil {
			self.recv_cond.Wait() //wait for more data available.
			self.recv_cond.L.Unlock()
		} else {
			self.recv_cond.L.Unlock()
			break
		}
	}

	if self.closed {
		return nil, errors.New("connection closed!")
	} else {
		return msg, nil
	}
}

func (self *recv_flow) read_message() []byte {
	var end *list.Element

	for i := self.ordered_recv_buf.Front(); i != nil; i = i.Next() {

		chunk := i.Value.(*data_chunk)

		if chunk.fragCtrl == fc_whole {
			{
				end = i
				break
			}
		} else if chunk.fragCtrl == fc_end {
			end = i
			break
		}
	}

	if end != nil {

		reach_end := false

		msg := bytes.NewBuffer(nil)
		for i := self.ordered_recv_buf.Front(); i != nil && !reach_end; {
			msg.Write(i.Value.(*data_chunk).data)

			if i == end {
				reach_end = true
			}

			bi := i
			i = i.Next()
			self.ordered_recved_bytes -= uint(len(bi.Value.(*data_chunk).data))
			self.ordered_recv_buf.Remove(bi)
		}

		return msg.Bytes()
	}

	return nil
}

func (self *recv_flow) on_userdata(fragmentControl uint8, sequenceNumber,
	fsnOffset uint, data, options []byte, abandon, final bool) {

	self.rx_data_packets++

	//fmt.Printf("recv_flow::on_userdata(%d-%d) last_ordered_seq#:%d\n", sequenceNumber, fsnOffset, self.last_ordered_seqnum)

	if self.recv_ranges.Contain(sequenceNumber) { //duplicate ack
		self.send_ack()
		return
	}

	ack_now := false

	if self.rx_data_packets >= 2 || //every other packet
		self.prev_rwnd < 2 || //previous receive windows notify least than 2*1024 bytes
		self.recv_ranges.ranges.Len() > 1 { // contain holes.

		ack_now = true
	}

	self.recv_ranges.AddRange(MakeRange(sequenceNumber, sequenceNumber+1))

	chunk := &data_chunk{
		fragCtrl: fragmentControl,
		seqNum:   sequenceNumber,
		data:     data,
	}

	self.unordered_recv_buf.push(chunk)
	self.unordered_recv_buf_bytes += uint(len(chunk.data))

	//try to order chunks
	for self.unordered_recv_buf.Len() > 0 {

		oldest_chunk := self.unordered_recv_buf.pop()

		if oldest_chunk.seqNum == self.last_ordered_seqnum+1 {
			self.ordered_recv_buf.PushBack(oldest_chunk)
			self.ordered_recved_bytes += uint(len(oldest_chunk.data))
			self.unordered_recv_buf_bytes -= uint(len(oldest_chunk.data))
			self.last_ordered_seqnum++

		} else {
			//restore ...
			self.unordered_recv_buf.push(oldest_chunk)
			break
		}
	}

	//notify can recv
	self.recv_cond.Signal()

	if ack_now {
		self.send_ack()
	} else {
		//delay send ack
		if self.delack_alarm == nil {
			self.delack_alarm = time.AfterFunc(200*time.Millisecond, func() {
				self.send_ack()
			})
		}
	}
}

func (self *recv_flow) send_ack() {

	self.rx_data_packets = 0
	self.prev_rwnd = int(self.available_buffers() / 1024)

	//remove delay ack alarm
	if self.delack_alarm != nil {
		self.delack_alarm.Stop()
		self.delack_alarm = nil
	}

	var recvRanges RangeQueue
	for _, c := range self.unordered_recv_buf.chunks {
		recvRanges.AddRange(MakeRange(c.seqNum, c.seqNum+1))
	}

	//fmt.Printf("send_ack(flowid:%d bufAvail:%d cumAck:%d recvRanges:%s)\n",
	//self.flowid, self.available_buffers(), self.last_ordered_seqnum, recvRanges.String())

	self.session.send_range_ack(self.flowid,
		self.available_buffers(), //bufAvail
		self.last_ordered_seqnum, //cumAck
		recvRanges.ToArray())
}

func (self *recv_flow) dump_state(w io.Writer) {
	fmt.Fprintln(w, "[RECV_FLOW]")
	fmt.Fprintf(w, "order_buf: %v\tunorder_buf: %v\t\n", self.ordered_recved_bytes, self.unordered_recv_buf_bytes)
	fmt.Fprintf(w, "received: %s\n", self.recv_ranges.String())
}
