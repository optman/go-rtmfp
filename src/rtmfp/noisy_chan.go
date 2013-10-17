package rtmfp

import (
	"container/list"
	"math/rand"
	"sync"
	"time"
)

type noisy_chan struct {
	in, out         chan *network_packet
	lose_rate       int //0-100
	delay           time.Duration
	capacity        int
	disorder        bool
	max_packet_size int
	speed           int // bytes/s

	packets *list.List

	send_mutex sync.Mutex
	send_cond  *sync.Cond

	packets_mutex sync.Mutex

	rx_count, tx_count, drop_count int

	bucket           int // bytes in one second
	bucket_inc_rate  int // bytes/ms,  us millionsecond to increase precise
	bucket_update_ts time.Time
}

type nosiy_chan_item struct {
	p           *network_packet
	queued_time time.Time
}

func (self *noisy_chan) open() {

	if self.in == nil {
		panic("in chan can't be nil")
	}

	self.out = make(chan *network_packet, network_packet_chan_default_buffer_size)
	self.packets = list.New()

	self.send_cond = sync.NewCond(&self.send_mutex)

	if self.speed > 0 {
		self.bucket = self.speed
		self.bucket_inc_rate = self.speed / 1000
		self.bucket_update_ts = time.Now()
	}

	go self.recv()
	go self.send()
}

func (self *noisy_chan) recv() {
	for {
		p, ok := <-self.in
		if !ok {
			break
		}
		self.recv_packet(p)

	}
}

func (self *noisy_chan) send() {
	for {

		self.send_cond.L.Lock()

		for self.packets.Len() == 0 {
			self.send_cond.Wait()
		}

		for self.packets.Len() > 0 {

			front := self.packets.Front()

			schedule_time := front.Value.(*nosiy_chan_item).queued_time.Add(self.delay)

			if time.Now().Before(schedule_time) {
				time.Sleep(schedule_time.Sub(time.Now()))
			}

			p := front.Value.(*nosiy_chan_item).p

			wait_time := self.update_send_bucket(len(p.data))
			if wait_time == 0 {
				self.send_packet(p)
				self.packets_mutex.Lock()
				self.packets.Remove(front)
				self.packets_mutex.Unlock()
			} else {
				time.Sleep(wait_time)
			}
		}

		self.send_cond.L.Unlock()
	}
}

func (self *noisy_chan) update_send_bucket(data_len int) time.Duration {

	//no speed limit
	if self.bucket_inc_rate == 0 {
		return 0
	}

	//increase bucket
	self.bucket += self.bucket_inc_rate * int(time.Since(self.bucket_update_ts)/time.Millisecond)
	self.bucket_update_ts = time.Now()
	if self.bucket > self.speed {
		self.bucket = self.speed
	}

	if self.bucket >= data_len {
		self.bucket -= data_len
		return 0
	} else {

		//may round to zero
		wait_time := (data_len - self.bucket) / self.bucket_inc_rate
		if wait_time == 0 {
			wait_time = 1 //force to 1 ms
		}

		return time.Duration(wait_time) * time.Millisecond
	}

}

func (self *noisy_chan) recv_packet(p *network_packet) {

	self.rx_count++
	self.drop_count++ //pre increase

	//drop by lose rate
	if rand.Int31n(100) < int32(self.lose_rate) {
		return
	}

	//drop by capacity
	if self.capacity > 0 && self.packets.Len() >= self.capacity {
		return
	}

	//drop by packet size
	if self.max_packet_size > 0 && len(p.data) > self.max_packet_size {
		return
	}

	self.drop_count-- //backoff

	item := &nosiy_chan_item{p: p, queued_time: time.Now()}


	//50% change of disorder
	self.packets_mutex.Lock()
	if self.disorder && rand.Int31n(100) < 50 && self.packets.Len() > 0 {
		self.packets.InsertBefore(item, self.packets.Back())
	} else {
		self.packets.PushBack(item)
	}
	self.packets_mutex.Unlock()

	self.send_cond.Signal()
}

func (self *noisy_chan) send_packet(p *network_packet) {
	self.tx_count++

	self.out <- p
}
