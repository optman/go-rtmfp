package rtmfp

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type chunk_handler interface {
	recv_ihello(srcAddr *string, edpType uint8, edpData, tag []byte)
	recv_fihello(srcAddr *string, edpType uint8, edpData []byte, replyAddress string, tag []byte)
	recv_rhello(srcAddr *string, tagEcho, cookie, respCert []byte)
	recv_rhello_cookie_change(srcAddr *string, oldCookie, newCookie []byte)
	recv_redirect(srcAddr *string, tagEcho []byte, redirectDestination []string)
	recv_iikeying(srcAddr *string, initSid uint32, cookieEcho, initCert, initNonce []byte)
	recv_rikeying(srcAddr *string, respSid uint32, respNonce []byte)
	recv_ping(srcAddr *string, msg []byte)
	recv_ping_reply(srcAddr *string, msgEcho []byte)
	recv_userdata(srcAddr *string, fragmentControl uint8, flowid, sequenceNumber, fsnOffset uint, data, options []byte, abandon, final bool)
	recv_range_ack(srcAddr *string, flowid, bufAvail, cumAck uint, recvRanges []Range)
	recv_buffer_probe(srcAddr *string, flowid uint)
	recv_flow_exception_report(srcAddr *string, flowid, exception uint)
	recv_session_close_request()
	recv_session_close_ack()
}

func decode_ihello_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[IHello Chunk]")

	edpType, edpData := decode_endpoint_discriminator(r)
	tag := r.Bytes()

	if handler != nil {
		handler.recv_ihello(srcAddr, edpType, edpData, tag)
	}

}

func decode_fihello_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[FIHello Chunk]")

	edpType, edpData := decode_endpoint_discriminator(r)
	replyAddress := decode_address(r)
	tag := r.Bytes()

	if handler != nil {
		handler.recv_fihello(srcAddr, edpType, edpData, replyAddress, tag)
	}

}

func decode_rhello_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[RHello Chunk]")

	tagLength := decode_vlu(r)
	tagEcho := r.Bytes()[:tagLength]
	r.Next(int(tagLength))

	cookieLength := decode_vlu(r)
	cookie := r.Bytes()[:cookieLength]
	r.Next(int(cookieLength))

	responderCertificate := r.Bytes()

	//fmt.Printf("tag:%v\ncookie:%v\n", tagEcho, cookie)

	//fmt.Println("responderCertificate:")
	//fmt.Println(responderCertificate)
	//dump_options(responderCertificate)

	if handler != nil {
		handler.recv_rhello(srcAddr, tagEcho, cookie, responderCertificate)
	}
}

func decode_rhello_cookie_change_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	fmt.Println("[RHello Cookie Change Chunk]")

	oldCookieLength := decode_vlu(r)
	oldCookie := r.Bytes()[:oldCookieLength]
	r.Next(int(oldCookieLength))

	newCookie := r.Bytes()

	if handler != nil{
		handler.recv_rhello_cookie_change(srcAddr, oldCookie, newCookie)
	}
}

func decode_redirect_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[Redirect Chunk]")

	tagLength := decode_vlu(r)
	tagEcho := r.Bytes()[:tagLength]
	r.Next(int(tagLength))

	redirectDestination := make([]string, 0)

	for r.Len() > 0 {
		redirectDestination = append(redirectDestination, decode_address(r))
	}

	fmt.Printf("redirectDestination:%v\n", redirectDestination)

	if handler != nil {
		handler.recv_redirect(srcAddr, tagEcho, redirectDestination)
	}
}

func decode_iikeying_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[IIKeying Chunk]")

	var initiatorSessionId uint32
	binary.Read(r, binary.BigEndian, &initiatorSessionId)

	cookieLength := decode_vlu(r)
	cookieEcho := r.Bytes()[:cookieLength]
	r.Next(int(cookieLength))

	certLength := decode_vlu(r)
	initiatorCertificate := r.Bytes()[:certLength]
	r.Next(int(certLength))

	//dh_pub_num := initiatorCertificate[len(initiatorCertificate)-128:]
	//fmt.Printf("initiator-dh-number:%v\n", dh_pub_num)

	skicLength := decode_vlu(r)
	sessionKeyInitiatorComponent := r.Bytes()[:skicLength]
	r.Next(int(skicLength))

	//signature := r.Bytes()

	//fmt.Printf("initiatorSessionId:%d\ncookieEcho:%v\nsignature:%v\n",
	//	initiatorSessionId, cookieEcho, signature)

	//fmt.Printf("initiatorCertificate:%d\n", certLength)
	//fmt.Println(initiatorCertificate)
	//dump_options(initiatorCertificate)

	//fmt.Printf("sessionKeyInitiatorComponent:%d\n", skicLength)
	//fmt.Println(sessionKeyInitiatorComponent)
	//dump_options(sessionKeyInitiatorComponent)

	if handler != nil {
		handler.recv_iikeying(srcAddr, initiatorSessionId, cookieEcho, initiatorCertificate, sessionKeyInitiatorComponent)
	}
}

func decode_rikeying_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[RIKeying Chunk]")

	var responderSessionId uint32
	binary.Read(r, binary.BigEndian, &responderSessionId)

	skrcLength := decode_vlu(r)
	sessionKeyResponderComponent := r.Bytes()[:skrcLength]
	r.Next(int(skrcLength))

	//signature := r.Bytes()

	//fmt.Printf("responderSessionId:%d\nsignature:%v\n",
	//	responderSessionId, signature)

	//fmt.Println("sessionKeyResponderComponent:")
	//fmt.Println(sessionKeyResponderComponent)
	//dump_options(sessionKeyResponderComponent)

	//
	//dh_pub_num := sessionKeyResponderComponent[len(sessionKeyResponderComponent)-128:]
	//fmt.Printf("resdoner-dh-number:%v\n", dh_pub_num)

	if handler != nil {
		handler.recv_rikeying(srcAddr, responderSessionId, sessionKeyResponderComponent)
	}
}

func decode_userdata_chunk(srcAddr *string, r *bytes.Buffer, cxt *packet_context, handler chunk_handler) {
	var flags uint8
	binary.Read(r, binary.BigEndian, &flags)

	var optionsPresent, abandon, final bool

	if (flags & 0x80) > 0 {
		optionsPresent = true
	}

	if (flags & 0x02) > 0 {
		abandon = true
	}

	if (flags & 0x01) > 0 {
		final = true
	}

	fragmentControl := (flags >> 4) & uint8(0x03)

	flowid := decode_vlu(r)
	sequenceNumber := decode_vlu(r)
	fsnOffset := decode_vlu(r)

	if cxt != nil {
		cxt.last_flowid = flowid
		cxt.last_seqnum = sequenceNumber
		cxt.last_fsnOffset = fsnOffset
	}

	var options []byte
	if optionsPresent {
		options = read_options(r)
	}

	data := r.Bytes()

	//fmt.Printf("[UserData Chunk]fragCtrl:%d  flowid:%d  seqNum:%d fsnOffset:%d options:%v\n",
	//	fragmentControl, flowid, sequenceNumber, fsnOffset, options)

	if handler != nil {
		handler.recv_userdata(srcAddr, fragmentControl, flowid, sequenceNumber, fsnOffset, data, options, abandon, final)
	}
}

func decode_next_userdata_chunk(srcAddr *string, r *bytes.Buffer, cxt *packet_context, handler chunk_handler) {
	var flags uint8
	binary.Read(r, binary.BigEndian, &flags)

	var optionsPresent, abandon, final bool

	if (flags & 0x80) > 0 {
		optionsPresent = true
	}

	if (flags & 0x02) > 0 {
		abandon = true
	}

	if (flags & 0x01) > 0 {
		final = true
	}

	fragmentControl := (flags >> 4) & uint8(0x03)

	if cxt != nil {
		cxt.last_seqnum++
		cxt.last_fsnOffset++
	}

	var options []byte
	if optionsPresent {
		options = read_options(r)
	}

	data := r.Bytes()

	//fmt.Printf("[NextUserData Chunk]fragCtrl:%d  flowid:%d  seqNum:%d fsnOffset:%d options:%v\n",
	//	fragmentControl, cxt.last_flowid, cxt.last_seqnum, cxt.last_fsnOffset, options)

	if handler != nil {
		handler.recv_userdata(srcAddr, fragmentControl, cxt.last_flowid, cxt.last_seqnum, cxt.last_fsnOffset, data, options, abandon, final)
	}
}

func decode_bitmap_ack_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	flowid := decode_vlu(r)
	bufAvail := decode_vlu(r) * 1024
	cumAck := decode_vlu(r)

	recvRanges := make([]Range, 0)
	//FIXME: decode bitmap and convert to recvRanges

	fmt.Printf("[BitmapACK Chunk]flowid:%d  bufAvail:%d  cumAck:%d  recvRanges:%v \n",
		flowid, bufAvail, cumAck, recvRanges)

	if handler != nil {
		handler.recv_range_ack(srcAddr, flowid, bufAvail, cumAck, recvRanges)
	}
}

func decode_range_ack_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {

	flowid := decode_vlu(r)
	bufAvail := decode_vlu(r) * 1024
	cumAck := decode_vlu(r)

	recvRanges := make([]Range, 0)
	ackCursor := cumAck + 1
	for r.Len() > 0 {
		pos := ackCursor + decode_vlu(r) + 1
		len := decode_vlu(r) + 1
		r := MakeRange(pos, pos+len)
		recvRanges = append(recvRanges, r)

		ackCursor = r.End()
	}

	//fmt.Printf("[RangeACK Chunk]flowid:%d  bufAvail:%d  cumAck:%d  recvRanges:%v \n",
	//	flowid, bufAvail, cumAck, RangeQueueFromArray(recvRanges).String())

	if handler != nil {
		handler.recv_range_ack(srcAddr, flowid, bufAvail, cumAck, recvRanges)
	}
}

func decode_session_close_request_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[SessionCloseRequst Chunk]")

	if handler != nil {
		handler.recv_session_close_request()
	}
}

func decode_session_close_ack_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[SessionCloseAck Chunk]")

	if handler != nil {
		handler.recv_session_close_ack()
	}
}

func decode_ping_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[Ping Chunk]")

	msg := r.Bytes()
	//fmt.Printf("msg:%v\n", msg)

	if handler != nil {
		handler.recv_ping(srcAddr, msg)
	}
}

func decode_ping_reply_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[PingReply Chunk]")

	msgEcho := r.Bytes()
	//fmt.Printf("msgEcho:%v\n", msgEcho)

	if handler != nil {
		handler.recv_ping_reply(srcAddr, msgEcho)
	}
}

func decode_buffer_probe_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[BufferProbe Chunk]")

	flowid := decode_vlu(r)

	if handler != nil {
		handler.recv_buffer_probe(srcAddr, flowid)
	}
}

func decode_flow_exception_report_chunk(srcAddr *string, r *bytes.Buffer, handler chunk_handler) {
	//fmt.Println("[FlowExceptionReport Chunk]")

	flowid := decode_vlu(r)
	exception := decode_vlu(r)

	if handler != nil {
		handler.recv_flow_exception_report(srcAddr, flowid, exception)
	}
}

func decode_chunk(srcAddr *string, chunk_type uint8, buf []byte, cxt *packet_context, handler chunk_handler) {
	r := bytes.NewBuffer(buf)

	switch chunk_type {
	case 0x30:
		decode_ihello_chunk(srcAddr, r, handler)
	case 0x0f:
		decode_fihello_chunk(srcAddr, r, handler)
	case 0x70:
		decode_rhello_chunk(srcAddr, r, handler)
	case 0x79:
		decode_rhello_cookie_change_chunk(srcAddr, r, handler)
	case 0x71:
		decode_redirect_chunk(srcAddr, r, handler)
	case 0x38:
		decode_iikeying_chunk(srcAddr, r, handler)
	case 0x78:
		decode_rikeying_chunk(srcAddr, r, handler)
	case 0x10:
		decode_userdata_chunk(srcAddr, r, cxt, handler)
	case 0x11:
		decode_next_userdata_chunk(srcAddr, r, cxt, handler)
	case 0x50:
		decode_bitmap_ack_chunk(srcAddr, r, handler)
	case 0x51:
		decode_range_ack_chunk(srcAddr, r, handler)
	case 0x0c:
		decode_session_close_request_chunk(srcAddr, r, handler)
	case 0x4c:
		decode_session_close_ack_chunk(srcAddr, r, handler)
	case 0x01:
		decode_ping_chunk(srcAddr, r, handler)
	case 0x41:
		decode_ping_reply_chunk(srcAddr, r, handler)
	case 0x18:
		decode_buffer_probe_chunk(srcAddr, r, handler)
	case 0x5e:
		decode_flow_exception_report_chunk(srcAddr, r, handler)
	default:
		fmt.Printf("#####################unknown chunk type:0x%x######################\n", chunk_type)
		panic("unknown chunk type!")
	}
}
