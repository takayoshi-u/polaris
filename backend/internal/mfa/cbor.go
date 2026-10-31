package mfa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// DecodeCBOR decodes a single CBOR data item from r.
func DecodeCBOR(r io.Reader) (any, error) {
	var header [1]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	major := header[0] >> 5
	info := header[0] & 0x1F

	var val uint64
	switch {
	case info < 24:
		val = uint64(info)
	case info == 24:
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		val = uint64(b[0])
	case info == 25:
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		val = uint64(binary.BigEndian.Uint16(b[:]))
	case info == 26:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		val = uint64(binary.BigEndian.Uint32(b[:]))
	case info == 27:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, err
		}
		val = binary.BigEndian.Uint64(b[:])
	default:
		return nil, fmt.Errorf("unsupported cbor additional info %d", info)
	}

	switch major {
	case 0: // Unsigned integer
		return int64(val), nil
	case 1: // Negative integer: -1 - val
		return -1 - int64(val), nil
	case 2: // Byte string
		buf := make([]byte, val)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf, nil
	case 3: // Text string
		buf := make([]byte, val)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf), nil
	case 4: // Array
		arr := make([]any, val)
		for i := 0; i < int(val); i++ {
			elem, err := DecodeCBOR(r)
			if err != nil {
				return nil, err
			}
			arr[i] = elem
		}
		return arr, nil
	case 5: // Map
		m := make(map[any]any, val)
		for i := 0; i < int(val); i++ {
			k, err := DecodeCBOR(r)
			if err != nil {
				return nil, err
			}
			v, err := DecodeCBOR(r)
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		return m, nil
	case 6: // Tag - decode tag and content
		return DecodeCBOR(r)
	case 7: // Simple / Float
		if info == 20 {
			return false, nil
		}
		if info == 21 {
			return true, nil
		}
		if info == 22 {
			return nil, nil
		}
		return nil, nil
	default:
		return nil, errors.New("unknown cbor major type")
	}
}

// UnmarshalCBOR parses cbor-encoded bytes into a Go value.
func UnmarshalCBOR(data []byte) (any, error) {
	return DecodeCBOR(bytes.NewReader(data))
}

func encodeHeader(w io.Writer, major byte, val uint64) error {
	switch {
	case val < 24:
		_, err := w.Write([]byte{(major << 5) | byte(val)})
		return err
	case val <= 0xFF:
		_, err := w.Write([]byte{(major << 5) | 24, byte(val)})
		return err
	case val <= 0xFFFF:
		var buf [3]byte
		buf[0] = (major << 5) | 25
		binary.BigEndian.PutUint16(buf[1:], uint16(val))
		_, err := w.Write(buf[:])
		return err
	case val <= 0xFFFFFFFF:
		var buf [5]byte
		buf[0] = (major << 5) | 26
		binary.BigEndian.PutUint32(buf[1:], uint32(val))
		_, err := w.Write(buf[:])
		return err
	default:
		var buf [9]byte
		buf[0] = (major << 5) | 27
		binary.BigEndian.PutUint64(buf[1:], val)
		_, err := w.Write(buf[:])
		return err
	}
}

// EncodeCBOR encodes val into writer w in CBOR format.
func EncodeCBOR(w io.Writer, val any) error {
	if val == nil {
		_, err := w.Write([]byte{0xF6}) // null
		return err
	}

	switch v := val.(type) {
	case bool:
		if v {
			_, err := w.Write([]byte{0xF5})
			return err
		}
		_, err := w.Write([]byte{0xF4})
		return err

	case int:
		if v >= 0 {
			return encodeHeader(w, 0, uint64(v))
		}
		return encodeHeader(w, 1, uint64(-1-v))

	case int64:
		if v >= 0 {
			return encodeHeader(w, 0, uint64(v))
		}
		return encodeHeader(w, 1, uint64(-1-v))

	case uint:
		return encodeHeader(w, 0, uint64(v))

	case uint64:
		return encodeHeader(w, 0, v)

	case []byte:
		if err := encodeHeader(w, 2, uint64(len(v))); err != nil {
			return err
		}
		_, err := w.Write(v)
		return err

	case string:
		b := []byte(v)
		if err := encodeHeader(w, 3, uint64(len(b))); err != nil {
			return err
		}
		_, err := w.Write(b)
		return err

	case []any:
		if err := encodeHeader(w, 4, uint64(len(v))); err != nil {
			return err
		}
		for _, elem := range v {
			if err := EncodeCBOR(w, elem); err != nil {
				return err
			}
		}
		return nil

	case map[any]any:
		if err := encodeHeader(w, 5, uint64(len(v))); err != nil {
			return err
		}
		for k, val := range v {
			if err := EncodeCBOR(w, k); err != nil {
				return err
			}
			if err := EncodeCBOR(w, val); err != nil {
				return err
			}
		}
		return nil

	case map[string]any:
		if err := encodeHeader(w, 5, uint64(len(v))); err != nil {
			return err
		}
		for k, val := range v {
			if err := EncodeCBOR(w, k); err != nil {
				return err
			}
			if err := EncodeCBOR(w, val); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported type for CBOR encoding: %T", val)
	}
}

// MarshalCBOR encodes val into CBOR bytes.
func MarshalCBOR(val any) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncodeCBOR(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

