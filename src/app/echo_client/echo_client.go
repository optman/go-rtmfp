package main

import (
	"../../rtmfp"
	"encoding/hex"
	"flag"
	"fmt"
)

var (
	addr_str       = flag.String("host", "127.0.0.1:8000", "remote addr/port")
	peerid_hex_str = flag.String("peerid", "", "peerid hexstr")
)

func main() {

	flag.Parse()

	var t rtmfp.Transport
	t.Open(":0", nil)

	peerid, _ := hex.DecodeString(*peerid_hex_str)
	stream, err := t.CreateBiStream(*addr_str, peerid)
	if err != nil {
		fmt.Printf("connect to %s fail! %s\n", *addr_str, err)
		return
	}

	go func() {
		for {
			data, err := stream.Recv()
			if err != nil {
				fmt.Println(err)
				break
			}

			fmt.Printf("<- %s\n", string(data))
		}
	}()

	for {
		var line string
		fmt.Scanln(&line)

		err := stream.Send([]byte(line))
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}
