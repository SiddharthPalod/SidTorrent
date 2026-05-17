package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func Decode(r *bufio.Reader) (interface{}, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case b >= '0' && b <= '9':
		return decodeString(r, b)
	case b == 'i':
		return decodeInt(r)
	case b == 'l':
		return decodeList(r)
	case b == 'd':
		return decodeDict(r)
	default:
		return nil, fmt.Errorf("unknown token: %c", b)
	}
}

func decodeString(r *bufio.Reader, first byte) (string, error) {
	lengthStr := string(first)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == ':' {
			break
		}
		lengthStr += string(b)
	}
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	return string(buf), err
}

func decodeInt(r *bufio.Reader) (int, error) {
	s := ""
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == 'e' {
			break
		}
		s += string(b)
	}
	return strconv.Atoi(s)
}

func decodeList(r *bufio.Reader) ([]interface{}, error) {
	var result []interface{}
	for {
		b, err := r.Peek(1)
		if err != nil {
			return nil, err
		}
		if b[0] == 'e' {
			_, _ = r.ReadByte()
			break
		}
		val, err := Decode(r)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

func decodeDict(r *bufio.Reader) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for {
		b, err := r.Peek(1)
		if err != nil {
			return nil, err
		}
		if b[0] == 'e' {
			_, _ = r.ReadByte()
			break
		}
		keyVal, err := Decode(r)
		if err != nil {
			return nil, err
		}
		key, ok := keyVal.(string)
		if !ok {
			return nil, errors.New("dictionary key must be string")
		}
		val, err := Decode(r)
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}
