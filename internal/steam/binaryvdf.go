package steam

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

const (
	kvTypeNone   = 0x00
	kvTypeString = 0x01
	kvTypeInt32  = 0x02
	kvTypeFloat  = 0x03
	kvTypeUint64 = 0x07
	kvTypeEnd    = 0x08
)

type kvValue any

type kvReader struct {
	r *bytes.Reader
}

func newKVReader(data []byte) *kvReader {
	return &kvReader{r: bytes.NewReader(data)}
}

func (kr *kvReader) readByte() (byte, error) {
	b, err := kr.r.ReadByte()
	if err != nil {
		return 0, err
	}
	return b, nil
}

func (kr *kvReader) readCString() (string, error) {
	var buf []byte
	for {
		b, err := kr.readByte()
		if err != nil {
			return "", err
		}
		if b == 0x00 {
			return string(buf), nil
		}
		buf = append(buf, b)
	}
}

func (kr *kvReader) readValue(t byte) (kvValue, error) {
	switch t {
	case kvTypeNone:
		obj := map[string]kvValue{}
		for {
			bt, err := kr.readByte()
			if err != nil {
				return nil, err
			}
			if bt == kvTypeEnd {
				return obj, nil
			}
			key, err := kr.readCString()
			if err != nil {
				return nil, err
			}
			val, err := kr.readValue(bt)
			if err != nil {
				return nil, err
			}
			obj[key] = val
		}
	case kvTypeString:
		return kr.readCString()
	case kvTypeInt32:
		var v int32
		if err := binary.Read(kr.r, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
		return v, nil
	case kvTypeFloat:
		var v float32
		if err := binary.Read(kr.r, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
		return v, nil
	case kvTypeUint64:
		var v uint64
		if err := binary.Read(kr.r, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported kv type: %d", t)
	}
}

func ReadBinaryVDF(data []byte) (map[string]kvValue, error) {
	kr := newKVReader(data)
	obj := map[string]kvValue{}
	for {
		bt, err := kr.readByte()
		if err != nil {
			if err == io.EOF {
				return obj, nil
			}
			return nil, err
		}
		if bt == kvTypeEnd {
			return obj, nil
		}
		key, err := kr.readCString()
		if err != nil {
			return nil, err
		}
		val, err := kr.readValue(bt)
		if err != nil {
			return nil, err
		}
		obj[key] = val
	}
}

type kvWriter struct {
	buf *bytes.Buffer
}

func newKVWriter() *kvWriter {
	return &kvWriter{buf: &bytes.Buffer{}}
}

func (kw *kvWriter) writeByte(b byte) {
	_ = kw.buf.WriteByte(b)
}

func (kw *kvWriter) writeCString(s string) {
	kw.buf.WriteString(s)
	kw.writeByte(0x00)
}

func (kw *kvWriter) writeValue(key string, val kvValue) error {
	switch v := val.(type) {
	case map[string]kvValue:
		kw.writeByte(kvTypeNone)
		kw.writeCString(key)
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := kw.writeValue(k, v[k]); err != nil {
				return err
			}
		}
		kw.writeByte(kvTypeEnd)
	case string:
		kw.writeByte(kvTypeString)
		kw.writeCString(key)
		kw.writeCString(v)
	case int32:
		kw.writeByte(kvTypeInt32)
		kw.writeCString(key)
		if err := binary.Write(kw.buf, binary.LittleEndian, v); err != nil {
			return err
		}
	case uint64:
		kw.writeByte(kvTypeUint64)
		kw.writeCString(key)
		if err := binary.Write(kw.buf, binary.LittleEndian, v); err != nil {
			return err
		}
	case float32:
		kw.writeByte(kvTypeFloat)
		kw.writeCString(key)
		if err := binary.Write(kw.buf, binary.LittleEndian, v); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported value type for key %s", key)
	}
	return nil
}

func WriteBinaryVDF(obj map[string]kvValue) ([]byte, error) {
	kw := newKVWriter()
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := kw.writeValue(k, obj[k]); err != nil {
			return nil, err
		}
	}
	kw.writeByte(kvTypeEnd)
	return kw.buf.Bytes(), nil
}
