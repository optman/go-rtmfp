package rtmfp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
)

var default_crypto_key = []byte("Adobe Systems 02")

type packet_handler interface {
	recv_packet_info(srcAddr *string, sid uint32, timeCritical, timeCriticalReverse bool,
		mode uint8, timestamp, timestampEcho uint16)
}

func calc_check_sum(buf []byte) uint16 {
	r := bytes.NewBuffer(buf)
	sum := uint32(0)

	for r.Len() > 0 {
		if r.Len() == 1 {
			var v uint8
			binary.Read(r, binary.BigEndian, &v)
			sum += uint32(v)
		} else {
			var v uint16
			binary.Read(r, binary.BigEndian, &v)
			sum += uint32(v)
		}
	}

	sum = (sum >> 16) + (sum & 0xffff)
	sum += (sum >> 16)

	return uint16(^sum)
}

type packet_context struct {
	last_flowid, last_seqnum, last_fsnOffset uint
}

func decode_packet(src_addr *string, buf, crypt_key []byte, packet_handler packet_handler, chunk_handler chunk_handler) {

	cxt := &packet_context{}

	r := bytes.NewBuffer(buf)

	var scrambleSessionId uint32
	var first32 [2]uint32

	binary.Read(r, binary.BigEndian, &scrambleSessionId)
	binary.Read(r, binary.BigEndian, &first32[0])
	binary.Read(r, binary.BigEndian, &first32[1])

	//fmt.Printf("ScrambledSessionID: %d\n", scrambleSessionId)

	sessionId := scrambleSessionId ^ first32[0] ^ first32[1]

	//fmt.Printf("SessionID: %d\n", sessionId)

	aes_key := crypt_key
	if aes_key == nil || sessionId == 0 {
		aes_key = default_crypto_key
	}

	if len(aes_key) != 16 {
		panic("invalid aes key")
	}

	c, err := aes.NewCipher([]byte(aes_key))
	if err != nil {
		fmt.Println(err)
		return
	}
	iv := make([]byte, 16)
	decrypt := cipher.NewCBCDecrypter(c, iv)

	packet := buf[4:]

	//fmt.Println("encrypted packet:")
	//fmt.Println(packet)

	decrypt.CryptBlocks(packet, packet)

	//fmt.Println("decrypted packet:")
	//fmt.Println(packet)

	r = bytes.NewBuffer(packet)

	var check_sum uint16
	binary.Read(r, binary.BigEndian, &check_sum)
	//fmt.Printf("0x%x\n", check_sum)

	calc_check_sum := calc_check_sum(r.Bytes())

	if check_sum != calc_check_sum {
		fmt.Println("###########check sum don't match!##########")
		//panic("check sum don't match!")
		return
	}

	var flags uint8
	binary.Read(r, binary.BigEndian, &flags)
	//fmt.Printf("Flag:%d\n", flags)

	time_critical := (flags & 128) > 0
	time_critical_reverse := (flags & 64) > 0
	time_stamp_present := (flags & 8) > 0
	time_stamp_echo_present := (flags & 4) > 0
	mode := (flags & 3)

	//fmt.Printf("time_critical:%v\ntime_critical_reverse:%v\ntime_stamp_present:%v\ntime_stamp_echo_present:%v\nmode:%d\n",
	//	time_critical, time_critical_reverse, time_stamp_present, time_stamp_echo_present, mode)

	var time_stamp, time_stamp_echo uint16

	if time_stamp_present {
		binary.Read(r, binary.BigEndian, &time_stamp)
		//fmt.Printf("time_stamp:%d\n", time_stamp)
	}

	if time_stamp_echo_present {
		binary.Read(r, binary.BigEndian, &time_stamp_echo)
		//fmt.Printf("time_stamp_echo:%d\n", time_stamp_echo)
	}

	if packet_handler != nil {
		packet_handler.recv_packet_info(src_addr,
			sessionId, time_critical, time_critical_reverse, mode, time_stamp, time_stamp_echo)
	}

	for r.Len() > 2 {

		var chunk_type uint8
		var chunk_length uint16

		binary.Read(r, binary.BigEndian, &chunk_type)
		binary.Read(r, binary.BigEndian, &chunk_length)

		if r.Len() < int(chunk_length) {
			break
		}

		//fmt.Printf("chunk(type:0x%x length:%d)\n", chunk_type, chunk_length)
		decode_chunk(src_addr, chunk_type, r.Bytes()[:chunk_length], cxt, chunk_handler)

		r.Next(int(chunk_length))
	}

}

type overwrite_buffer struct {
	buf []byte
}

func (self *overwrite_buffer) Write(p []byte) (n int, err error) {

	copy(self.buf, p)

	return len(p), nil
}

func new_overwrite_buffer(buf []byte) io.Writer {

	return &overwrite_buffer{
		buf: buf,
	}
}

type packet struct {
	time_critical, time_critical_reverse bool
	time_stamp, time_stamp_echo          uint16
	mode                                 uint8
	buf                                  *bytes.Buffer
}

func (self *packet) init() {
	self.buf = bytes.NewBuffer(nil)

	binary.Write(self.buf, binary.BigEndian, uint32(0)) //session id

	binary.Write(self.buf, binary.BigEndian, uint16(0)) //checksum

	flags := uint8(0)

	if self.time_critical {
		flags |= 128
	}

	if self.time_critical_reverse {
		flags |= 64
	}

	if self.time_stamp > 0 {
		flags |= 8
	}

	if self.time_stamp_echo > 0 {
		flags |= 4
	}

	flags |= self.mode

	binary.Write(self.buf, binary.BigEndian, flags)

	if self.time_stamp > 0 {
		binary.Write(self.buf, binary.BigEndian, self.time_stamp)
	}

	if self.time_stamp_echo > 0 {
		binary.Write(self.buf, binary.BigEndian, self.time_stamp_echo)
	}
}

func (self *packet) add_chunk(chunk_type uint8, data []byte) {
	binary.Write(self.buf, binary.BigEndian, chunk_type)
	binary.Write(self.buf, binary.BigEndian, uint16(len(data)))
	self.buf.Write(data)
}

func (self *packet) pack(session_id uint32, crypt_key []byte) []byte {

	encrypted_len := (self.buf.Len() - 4) //exclude session id
	padding_len := ((encrypted_len-1)/16+1)*16 - encrypted_len

	for i := 0; i < padding_len; i++ {
		binary.Write(self.buf, binary.BigEndian, uint8(0xff))
	}

	raw_data := self.buf.Bytes()

	check_sum := calc_check_sum(raw_data[6:])

	binary.Write(new_overwrite_buffer(raw_data[4:]), binary.BigEndian, check_sum)

	packet := raw_data[4:]

	aes_key := crypt_key
	if aes_key == nil || session_id == 0 {
		aes_key = default_crypto_key
	}

	c, err := aes.NewCipher(aes_key)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	iv := make([]byte, 16)
	decrypt := cipher.NewCBCEncrypter(c, iv)
	decrypt.CryptBlocks(packet, packet)

	var first32 [2]uint32
	read_buf := bytes.NewBuffer(raw_data[4:])
	binary.Read(read_buf, binary.BigEndian, &first32[0])
	binary.Read(read_buf, binary.BigEndian, &first32[1])

	scrambled_session_id := session_id ^ first32[0] ^ first32[1]

	binary.Write(new_overwrite_buffer(raw_data), binary.BigEndian, scrambled_session_id)

	return raw_data
}
