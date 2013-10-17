package rtmfp

import (
	"bytes"
	"testing"
)

func TestVlu(t *testing.T) {

	buf :=	bytes.NewBuffer(nil) 

	v := uint(1)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}

	v = uint(9)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}

	v = uint(128)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}

	v = uint(129)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}

	v = uint(256)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}

	v = uint(3434333333)
	encode_vlu(buf, v) 
	if v != decode_vlu(buf){
		t.Fatal()
	}
}
