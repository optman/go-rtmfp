package rtmfp

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	//	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"
)

type data_chunk struct {
	fragCtrl   uint8
	seqNum     uint
	data       []byte
	send_count int
	in_flight  bool
	tsn        int
	nak_count  int
}

func gen_peerid_from_cert(cert []byte) []byte {
	hash := sha256.New()
	hash.Write(cert)
	return hash.Sum(nil)
}

//unit is 4 milliseconds
func timestamp() uint16 {
	return uint16(time.Now().UnixNano() / (4 * 1000000))
}

func HmacSha256(a, b []byte) []byte {

	h := hmac.New(sha256.New, a)
	h.Write(b)
	return h.Sum(nil)
}

func GenCryptoKeys(share_secret *big.Int, myNonce, otherNonce []byte) (dkey, ekey []byte) {
	dkey = HmacSha256(share_secret.Bytes(), HmacSha256(myNonce, otherNonce))[:16]
	ekey = HmacSha256(share_secret.Bytes(), HmacSha256(otherNonce, myNonce))[:16]

	//fmt.Printf("dkey:%v\nekey:%v\n", dkey, ekey)
	return
}

func get_vlu_size(uint) uint {
	//FIXME: .....
	return 1
}

func decode_vlu(r io.Reader) uint {

	vlu_value := uint(0)

	for {
		var v uint8
		binary.Read(r, binary.BigEndian, &v)

		vlu_value = vlu_value*128 + uint(v&0x7f)

		if (v & 128) == 0 {
			break
		}
	}

	return vlu_value
}

func encode_vlu(w io.Writer, v uint) {

	internal_encode_vlu(w, v, false)
}

func encode_vlu_prefix_bytes(w io.Writer, buf []byte) {
	encode_vlu(w, uint(len(buf)))
	w.Write(buf)
}

func internal_encode_vlu(w io.Writer, v uint, more bool) {

	if v >= 128 {
		internal_encode_vlu(w, v/128, true)
		internal_encode_vlu(w, v%128, more)
	} else {

		v7bit := uint8(v)
		if more {
			v7bit |= 128
		}

		binary.Write(w, binary.BigEndian, v7bit)
	}
}

func dump_options(buf []byte) {
	r := bytes.NewBuffer(buf)

	for r.Len() > 0 {

		len := decode_vlu(r)

		var opt_type uint8
		binary.Read(r, binary.BigEndian, &opt_type)

		data_len := int(len - 1)

		fmt.Printf("opt(len:%d type:0x%x value:%v)\n", data_len, opt_type, r.Bytes()[:data_len])

		r.Next(data_len)
	}
}

func read_options(r io.Reader) []byte {

	options_buf := bytes.NewBuffer(nil)

	for {

		len := decode_vlu(r)
		encode_vlu(options_buf, len)

		if len == 0 {
			break
		}

		data := make([]byte, len)
		r.Read(data)

		options_buf.Write(data)
	}

	return options_buf.Bytes()
}

func read_option(buf []byte, opt_type uint8) []byte {

	r := bytes.NewBuffer(buf)

	for r.Len() > 0 {

		len := decode_vlu(r)
		data_len := int(len - 1)

		var this_opt_type uint8
		binary.Read(r, binary.BigEndian, &this_opt_type)

		if opt_type == this_opt_type {
			return r.Bytes()[:data_len]
		}

		r.Next(data_len)
	}

	return nil
}

func read_vlu_option(buf []byte, opt_type uint8, default_value uint) uint {

	r := bytes.NewBuffer(buf)

	for r.Len() > 0 {

		len := decode_vlu(r)

		var this_opt_type uint8
		binary.Read(r, binary.BigEndian, &this_opt_type)

		if opt_type == this_opt_type {
			return decode_vlu(r)
		}

		data_len := int(len - 1)
		r.Next(data_len)
	}

	return default_value
}

func decode_endpoint_discriminator(r *bytes.Buffer) (edpType uint8, edpData []byte) {

	/*epdLength := */
	decode_vlu(r)
	edpLength := decode_vlu(r)

	binary.Read(r, binary.BigEndian, &edpType)

	edpData_len := edpLength - 1
	edpData = r.Bytes()[:edpData_len]
	if edpType == 0xf {
		//fmt.Printf("edp(type:0x%x date:%s)\n", edpType, hex.EncodeToString(edpData))
	} else {
		//fmt.Printf("edp(type:0x%x url:%s)\n", edpType, edpData)
	}

	r.Next(int(edpData_len))

	return
}

func decode_address(r *bytes.Buffer) string {

	var flag uint8
	binary.Read(r, binary.BigEndian, &flag)

	var ipAddress net.IP
	if flag&0x80 == 0 {
		ipAddress = r.Bytes()[0:4] //ipv4
		r.Next(4)
	} else {
		ipAddress = r.Bytes()[0:16] //ipv6
		r.Next(16)
	}

	var port uint16
	binary.Read(r, binary.BigEndian, &port)

	addr := &net.UDPAddr{
		IP:   ipAddress,
		Port: int(port),
	}

	return addr.String()
}

func min_duration(a, b time.Duration) time.Duration {
	if a > b {
		return b
	} else {
		return a
	}
}

func max_duration(a, b time.Duration) time.Duration {
	if a < b {
		return b
	} else {
		return a
	}
}

func min_uint(a, b uint) uint {
	if a > b {
		return b
	} else {
		return a
	}
}

func max_uint(a, b uint) uint {
	if a < b {
		return b
	} else {
		return a
	}
}
