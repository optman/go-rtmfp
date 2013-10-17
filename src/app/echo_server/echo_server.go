package main

import (
	"../../rtmfp"
	"encoding/hex"
	"flag"
	"fmt"
)

var (
	addr_str      = flag.String("listen", "0.0.0.0:8000", "bind addr/port")
	pseudo_id_str = flag.String("id", "123456", "pseudo_id")
)

func main() {

	flag.Parse()

	var t rtmfp.Transport

	t.SetStreamHandler(handle_stream)

	err := t.Open(*addr_str, []byte(*pseudo_id_str))
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("listen: %s\npeerid: %s\n", t.LocalAddr(), hex.EncodeToString(t.Peerid()))

	select {}
}

func handle_stream(s *rtmfp.BiStream, addr string) bool {

	go func() {

		for {
			data, err := s.Recv()
			if err != nil {
				fmt.Println(err)
				break
			} else {
				s.Send(data)
			}
		}

	}()

	return true
}
