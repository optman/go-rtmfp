package rtmfp

import (
	"testing"
	"time"

//	"fmt"
)

func TestTransport(t *testing.T) {

	s := &Transport{}
	s.SetStreamHandler(func(stream *BiStream, addr string) bool {

		go func() {
			data, _ := stream.Recv()
			stream.Send(data)
		}()
		return true
	})
	s.Open("127.0.0.1:0", []byte("abc"))

	done := make(chan bool, 1)

	c := &Transport{}
	c.SetStreamHandler(func(*BiStream, string) bool { return false })
	c.Open(":0", []byte("efg"))

	msg := "hello"
	stream, err := c.CreateBiStream(s.LocalAddr(), s.Peerid())

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		stream.Send([]byte(msg))

		data, _ := stream.Recv()
		if string(data) != msg {
			t.Fatal("echo msg not match!")
		}

		done <- true
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
}
