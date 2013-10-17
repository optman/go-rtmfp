package rtmfp


import (
)

func CreateDummySession(addr, edp string) error{

	bin := socket_bin{}
	bin.open(":0")

	hs := handshake{
		in:  bin.out,
		out: bin.in,
	}
	hs.open()

	_, err := hs.create_session(addr, []byte(edp))

	bin.close()
	hs.close()

	return err
}
