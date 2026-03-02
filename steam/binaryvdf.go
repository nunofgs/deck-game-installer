// Package steam provides Steam integration for shortcuts, configuration, and process management.
package steam

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// Binary VDF type constants
const (
	kvTypeNone   = 0x00
	kvTypeString = 0x01
	kvTypeInt32  = 0x02
	kvTypeFloat  = 0x03
	kvTypeUint64 = 0x07
	kvTypeEnd    = 0x08
)

// KVValue represents a value in a binary VDF structure.
// Can be string, int32, float32, uint64, or map[string]KVValue.
type KVValue any

// kvReader reads binary VDF data.
type kvReader struct {
	r *bytes.Reader
}

// newKVReader creates a new reader for the given binary data.
func newKVReader(data []byte) *kvReader {
	return &kvReader{r: bytes.NewReader(data)}
}

// readByte reads a single byte.
func (kr *kvReader) readByte() (byte, error) {
	return kr.r.ReadByte()
}

// readCString reads a null-terminated string.
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

// readValue reads a value of the specified type.
func (kr *kvReader) readValue(t byte) (KVValue, error) {
	switch t {
	case kvTypeNone:
		// Nested object
		obj := map[string]KVValue{}
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
		return nil, fmt.Errorf("unsupported binary VDF type: %d", t)
	}
}

// ReadBinaryVDF parses binary VDF data (used for shortcuts.vdf).
func ReadBinaryVDF(data []byte) (map[string]KVValue, error) {
	kr := newKVReader(data)
	obj := map[string]KVValue{}

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

// kvWriter writes binary VDF data.
type kvWriter struct {
	buf *bytes.Buffer
}

// newKVWriter creates a new binary VDF writer.
func newKVWriter() *kvWriter {
	return &kvWriter{buf: &bytes.Buffer{}}
}

// writeByte writes a single byte.
func (kw *kvWriter) writeByte(b byte) {
	_ = kw.buf.WriteByte(b)
}

// writeCString writes a null-terminated string.
func (kw *kvWriter) writeCString(s string) {
	kw.buf.WriteString(s)
	kw.writeByte(0x00)
}

// writeValue writes a key-value pair.
func (kw *kvWriter) writeValue(key string, val KVValue) error {
	switch v := val.(type) {
	case map[string]KVValue:
		kw.writeByte(kvTypeNone)
		kw.writeCString(key)

		// Sort keys for consistent output
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
		return fmt.Errorf("unsupported value type for key %s: %T", key, val)
	}
	return nil
}

// WriteBinaryVDF serializes a map to binary VDF format.
func WriteBinaryVDF(obj map[string]KVValue) ([]byte, error) {
	kw := newKVWriter()

	// Sort keys for consistent output
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
