package rtmfp

/*
import (
	//	"fmt"
	"encoding/hex"
	"testing"
	"time"
)

func TestSocketBinClient(t *testing.T) {

	bin := socket_bin{}
	bin.open(":0")

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	var initiator *session

	peerid, _ := hex.DecodeString("33319f62ad453f3b5690806c7d1c0b2b7c26da97edf733eeaf00a17488626fb3")

	var err error
	initiator, err = hs.create_session("127.0.0.1:1935", peerid)

	if err != nil {
		t.Fatal(err)
	}

	initiator.send_ping()

	//wait for ping reply
	time.Sleep(100 * time.Millisecond)

	bin.close()
	hs.close()
}
*/