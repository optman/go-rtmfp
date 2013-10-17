package rtmfp

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestAMF(t *testing.T) {

	obj := make(map[string]interface{})
	obj["string"] = "abc"
	obj["number"] = float64(2.5)
	obj["bool"] = true
	obj["null"] = nil

	bs := make([]byte, 10)
	rand.Read(bs)
	obj["bytearray"] = bs

	buf := bytes.NewBuffer(nil)
	encode_amf(buf, obj)

	obj2 := decode_amf(buf).(map[string]interface{})

	if obj["string"].(string) != obj2["string"].(string) ||
		obj["number"].(float64) != obj2["number"].(float64) ||
		obj["bool"].(bool) != obj2["bool"].(bool) ||
		obj["null"] != obj2["null"] ||
		!bytes.Equal(obj["bytearray"].([]byte), obj2["bytearray"].([]byte)) {
		t.Fatal("decode fail.")
	}

}

func test_u29(v uint, t *testing.T) {
	buf := bytes.NewBuffer(nil)
	write_amf3_u29(buf, v)
	if v != read_amf3_u29(buf) {
		t.Fatal("not match.")
	}
}

func TestAMF3U29(t *testing.T) {
	test_u29(0xF, t)
	test_u29(0xFFFF, t)
	test_u29(0xFFFFF, t)
	test_u29(0xFFFFFF, t)
	test_u29(0xFFFFFFF, t)
	test_u29(0x1FFFFFFF, t)

	test_u29(0x12345678, t)
}
