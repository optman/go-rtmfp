package rtmfp

import (
	"fmt"
	"testing"
	"time"
)

func test_noisy_chan(send_packet_count, lose_rate int, delay time.Duration, capacity int) (int, time.Duration) {
	in := make(chan *network_packet, network_packet_chan_default_buffer_size)

	nc := &noisy_chan{
		in:              in,
		lose_rate:       lose_rate,
		delay:           delay,
		capacity:        capacity,
		disorder:        true,
		max_packet_size: 1500,
	}
	nc.open()

	out := nc.out

	start_time := time.Now()

	go func() {
		for i := 0; i < send_packet_count; i++ {
			in <- &network_packet{}
		}
	}()

	var total_delay time.Duration

	recv_packet_cout := 0
	go func() {
		for {
			<-out
			recv_packet_cout++
			if send_packet_count == recv_packet_cout {
				total_delay = time.Since(start_time)
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	actual_lose_rate := (send_packet_count - recv_packet_cout) * 100 / send_packet_count

	fmt.Printf("send %d packets. lose_rate: %d%%  total_delay:%v\n", send_packet_count, actual_lose_rate, total_delay)

	return actual_lose_rate, total_delay
}

func TestNoisyChan(t *testing.T) {

	lr, _ := test_noisy_chan(1000, 75, 0, 0)
	if lr < 70 || lr > 80 {
		t.Fatal("lose rate not correct.")
	}

	lr, _ = test_noisy_chan(1000, 0, 10*time.Millisecond, 250)
	if lr < 70 || lr > 80 {
		t.Fatal("lose rate not correct.")
	}

	_, delay := test_noisy_chan(1000, 0, 10*time.Millisecond, 0)
	if delay < 5*time.Millisecond || delay > 15*time.Millisecond {
		t.Fatal("delay not correct.")
	}
}
