package rtmfp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func read_amf0_type_number(r *bytes.Buffer) float64 {

	data_type, _ := r.ReadByte()
	if data_type != 0x0 {
		panic("wrong data type")
	}

	return read_amf0_number(r)
}

func read_amf0_number(r *bytes.Buffer) float64 {

	var v float64
	binary.Read(r, binary.BigEndian, &v)

	return v
}

func write_amf0_type_number(w *bytes.Buffer, v float64) {
	w.WriteByte(0x0)
	write_amf0_number(w, v)
}

func write_amf0_number(w *bytes.Buffer, v float64) {
	binary.Write(w, binary.BigEndian, v)
}

func read_amf0_bool(r *bytes.Buffer) bool {

	var v uint8
	binary.Read(r, binary.BigEndian, &v)

	if v > 0 {
		return true
	} else {
		return false
	}
}

func write_amf0_bool(w *bytes.Buffer, v bool) {
	if v {
		binary.Write(w, binary.BigEndian, uint8(1))
	} else {
		binary.Write(w, binary.BigEndian, uint8(0))
	}
}

func read_amf0_type_string(r *bytes.Buffer) string {

	data_type, _ := r.ReadByte()
	if data_type != 0x2 {
		panic("wrong data type")
	}

	return read_amf0_string(r)
}

func read_amf0_string(r *bytes.Buffer) string {

	var str_len uint16
	binary.Read(r, binary.BigEndian, &str_len)

	str_data := make([]byte, str_len)
	r.Read(str_data)

	return string(str_data)
}

func write_amf0_type_string(w *bytes.Buffer, v string) {
	w.WriteByte(0x2)
	write_amf0_string(w, v)
}

func write_amf0_string(w *bytes.Buffer, v string) {
	binary.Write(w, binary.BigEndian, uint16(len(v)))
	w.Write([]byte(v))
}

func read_amf0_object(r *bytes.Buffer) (obj map[string]interface{}) {

	obj = make(map[string]interface{})

	for r.Bytes()[0] != 0x09 /*object end marker*/ {

		key := read_amf0_string(r)
		if len(key) == 0 {
			break
		}

		obj[key] = read_amf0(r)

		//fmt.Printf("%s:%v\n", key, obj[key])
	}

	r.Next(1) //object end marker

	return obj
}

func write_amf0_object(w *bytes.Buffer, obj map[string]interface{}) {

	for k, v := range obj {
		write_amf0_string(w, k)
		write_amf0(w, v)
	}
	write_amf0_string(w, "")
	w.WriteByte(0x09)
}

func write_amf0_type_null(w *bytes.Buffer) {
	w.WriteByte(0x5)
}

func read_amf0(r *bytes.Buffer) interface{} {

	data_type, _ := r.ReadByte()

	switch data_type {
	case 0x0:
		return read_amf0_number(r)
	case 0x1:
		return read_amf0_bool(r)
	case 0x2:
		return read_amf0_string(r)
	case 0x3:
		return read_amf0_object(r)
	case 0x5: //null
		return nil
	case 0x6: //undefined
		return nil
	case 0x11: //change to amf3
		return read_amf3(r)
	default:
		fmt.Printf("#unknown type:0x%x\n", data_type)
		panic("unknwo type")
	}

	return nil
}

func write_amf0(w *bytes.Buffer, val interface{}) {

	switch v := val.(type) {

	case float64:
		w.WriteByte(0x0)
		write_amf0_number(w, v)
	case bool:
		w.WriteByte(0x1)
		write_amf0_bool(w, v)
	case string:
		w.WriteByte(0x2)
		write_amf0_string(w, v)
	case map[string]interface{}:
		w.WriteByte(0x3)
		write_amf0_object(w, v)
	case nil:
		w.WriteByte(0x5)
	case []byte:
		w.WriteByte(0x11) //change to amf3
		write_amf3(w, val)
	default:
		fmt.Printf("unknown type: %v\n", v)
		panic("unknow type")
	}

}

func read_amf3_u29(r *bytes.Buffer) uint {

	u29_value := uint(0)

	for i := 0; i < 4; i++ {
		var v uint8
		binary.Read(r, binary.BigEndian, &v)

		if i < 3 {
			u29_value = u29_value*128 + uint(v&0x7f)
		} else {
			u29_value = u29_value*256 + uint(v)
		}

		if (v & 128) == 0 {
			break
		}
	}

	return u29_value
}

func write_amf3_u29(w *bytes.Buffer, v uint) {

	if v <= 0x1FFFFF {
		internal_write_amf3_u29(w, v, false)
	} else if v <= 0x1FFFFFFF {
		internal_write_amf3_u29(w, v/256, true)
		w.WriteByte(uint8(v % 256))
	} else {
		panic("exceed 2^29 - 1!")
	}
}

func internal_write_amf3_u29(w io.Writer, v uint, more bool) {

	if v >= 128 {
		internal_write_amf3_u29(w, v/128, true)
		internal_write_amf3_u29(w, v%128, more)
	} else {

		v7bit := uint8(v)
		if more {
			v7bit |= 128
		}

		binary.Write(w, binary.BigEndian, v7bit)
	}
}

func read_amf3_bytearray(r *bytes.Buffer) []byte {
	bytes := make([]byte, read_amf3_u29(r)/2)
	r.Read(bytes)
	return bytes
}

func write_amf3_bytearray(w *bytes.Buffer, v []byte) {
	write_amf3_u29(w, uint(len(v)*2+1))
	w.Write(v)

	//fmt.Printf("array:%v\n", v)
}

func read_amf3(r *bytes.Buffer) interface{} {
	data_type, _ := r.ReadByte()

	switch data_type {
	case 0x0c:
		return read_amf3_bytearray(r)
	default:
		fmt.Printf("#unknown type:0x%x\n", data_type)
		panic("unknwo type")
	}
}

func write_amf3(w *bytes.Buffer, val interface{}) {
	switch v := val.(type) {
	case []byte:
		w.WriteByte(0x0c)
		write_amf3_bytearray(w, v)
	default:
		fmt.Printf("unknown type: %v\n", v)
		panic("unknow type")
	}
}

func decode_amf(r *bytes.Buffer) interface{} {

	//fmt.Printf("decode amf: %v\n", r.Bytes())

	return read_amf0(r)
}

func encode_amf(w *bytes.Buffer, v interface{}) {

	write_amf0(w, v)

	//fmt.Printf("encode amf: %v\n", w.Bytes())
}
