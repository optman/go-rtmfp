package rtmfp

import (
	"encoding/hex"
	//	"fmt"
	"math/big"
	"os"
	"testing"
)

func TestParseData(t *testing.T) {

	path := "../../data/"

	var handler dummy_handler

	my_decode_packet(path+"IHello.dat", nil, &handler)
	my_decode_packet(path+"RHello.dat", nil, &handler)
	my_decode_packet(path+"IIKeying.dat", nil, &handler)
	my_decode_packet(path+"RIKeying.dat", nil, &handler)
	my_decode_packet(path+"5.dat", handler.dkey, &handler)
	my_decode_packet(path+"6.dat", handler.ekey, &handler)
	my_decode_packet(path+"7.dat", handler.dkey, &handler)
	my_decode_packet(path+"8.dat", handler.dkey, &handler)
	my_decode_packet(path+"9.dat", handler.ekey, &handler)
	my_decode_packet(path+"10.dat", handler.dkey, &handler)
	my_decode_packet(path+"11.dat", handler.dkey, &handler)
}

func my_decode_packet(file_name string, crypt_key []byte, handler chunk_handler) {
	buf := make([]byte, 1024)

	fo, _ := os.Open(file_name)
	read_bytes, _ := fo.Read(buf)
	//fmt.Printf("read %d bytes\n", read_bytes)

	buf = buf[0:read_bytes]
	decode_packet(nil, buf, crypt_key, nil, handler)
}

func responderComputeKeys(other_dh_pub_num, initNonce []byte) (dkey, ekey []byte) {

	var dh1024p big.Int
	dh1024p.SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF", 16)

	//////////////////////////////////////////////////////////
	//use fixed number here!
	var my_x big.Int
	my_x.SetString("00FFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF", 16)

	my_pub := "\xEB\x93\x67\x70\x23\xC1\xFC\x12\x23\xE6\x36\x13\x4A\xA8\x39\xB9\xC3\x38\x91\x38\x08\x68\xC4\x75\x3D\x1A\x45\xDE\xEF\xC9\x2B\x92\x53\x46\xEF\x48\x27\x35\x34\x9E\x6D\x9A\x5B\xCA\x2C\xA2\x94\x07\x71\x0F\xAC\x39\x3C\x8A\x3C\x05\xAD\x95\x1A\x1F\xA1\xF3\xA2\xA4\xDF\xD6\x41\xA0\x6F\x73\x9B\xA8\x2C\x26\xAC\x45\xC0\x89\x19\xD8\xDA\xE4\x1A\xC2\x1E\xD6\xDE\x83\x29\x8A\xC9\x53\x5C\x91\x3E\xD6\x69\x6F\x50\x61\xC2\x7B\x6D\x5E\x42\x4D\xFE\x63\xA0\x9F\x78\x8E\x93\x51\xEE\x29\x53\x7B\xD6\xB4\xB9\xC0\xDF\xE1\xC9\x13\xD7\x62"

	my_nonce := []byte("\x03\x1A\x00\x00\x02\x1E\x00\x81\x02\x0D\x02" + my_pub)
	//////////////////////////////////////////////////////////

	//fmt.Printf("responderNonce:%v\n", my_nonce)

	var other_pub big.Int
	other_pub.SetString(hex.EncodeToString(other_dh_pub_num), 16)

	secret := my_x.Exp(&other_pub, &my_x, &dh1024p)

	return GenCryptoKeys(secret, my_nonce, initNonce)
}

type dummy_handler struct {
	dkey, ekey []byte
}

func (self *dummy_handler) recv_ihello(srcAddr *string, edpType uint8, edpData, tag []byte) {}
func (self *dummy_handler) recv_fihello(srcAddr *string, edpType uint8, edpData []byte, replyAddress string, tag []byte) {
}
func (self *dummy_handler) recv_rhello(srcAddr *string, tagEcho, cookie, respCert []byte) {}
func (self *dummy_handler) recv_redirect(srcAddr *string, tagEcho []byte, redirectDestination []string) {
}
func (self *dummy_handler) recv_rhello_cookie_change(srcAddr *string, oldCookie, newCookie []byte) {
}

func (self *dummy_handler) recv_iikeying(srcAddr *string, initSid uint32, cookieEcho, initCert, initNonce []byte) {
	self.dkey, self.ekey = responderComputeKeys(initCert[len(initCert)-128:], initNonce)
}
func (self *dummy_handler) recv_rikeying(srcAddr *string, respSid uint32, respNonce []byte) {}
func (self *dummy_handler) recv_ping(srcAddr *string, msg []byte)                           {}
func (self *dummy_handler) recv_ping_reply(srcAddr *string, msgEcho []byte)                 {}

func (self *dummy_handler) recv_userdata(srcAddr *string, fragmentControl uint8, flowid, sequenceNumber, fsnOffset uint, data, options []byte, abandon, final bool) {
}
func (self *dummy_handler) recv_range_ack(srcAddr *string, flowid, bufAvail, cumAck uint, recvRanges []Range) {
}
func (self *dummy_handler) recv_buffer_probe(srcAddr *string, flowid uint)                     {}
func (self *dummy_handler) recv_flow_exception_report(srcAddr *string, flowid, exception uint) {}

func (self *dummy_handler) recv_session_close_request() {}
func (self *dummy_handler) recv_session_close_ack()     {}
