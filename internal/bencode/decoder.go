package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Node struct {
	Value interface{}
	Start int
	End   int

	Dict map[string]*Node
	List []*Node
}

func Decode(r *bufio.Reader) (interface{}, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return DecodeBytes(data)
}

func DecodeWithRaw(data []byte) (*Node, error) {
	d := decoder{data: data}
	node, err := d.decodeValue()
	if err != nil {
		return nil, err
	}
	if d.pos != len(data) {
		return nil, errors.New("trailing data after bencode value")
	}
	return node, nil
}

type decoder struct {
	data []byte
	pos  int
}

func (d *decoder) decodeValue() (*Node, error) {
	if d.pos >= len(d.data) {
		return nil, io.ErrUnexpectedEOF
	}
	start := d.pos
	b := d.readByte()
	switch {
	case b >= '0' && b <= '9':
		return d.decodeString(start, b)
	case b == 'i':
		return d.decodeInt(start)
	case b == 'l':
		return d.decodeList(start)
	case b == 'd':
		return d.decodeDict(start)
	default:
		return nil, fmt.Errorf("unknown token: %c", b)
	}
}

func (d *decoder) decodeString(start int, first byte) (*Node, error) {
	lengthStr := []byte{first}
	for {
		if d.pos >= len(d.data) {
			return nil, io.ErrUnexpectedEOF
		}
		b := d.readByte()
		if b == ':' {
			break
		}
		if b < '0' || b > '9' {
			return nil, fmt.Errorf("invalid string length byte: %q", b)
		}
		lengthStr = append(lengthStr, b)
	}
	if len(lengthStr) > 1 && lengthStr[0] == '0' {
		return nil, errors.New("invalid string length with leading zero")
	}
	length, err := strconv.Atoi(string(lengthStr))
	if err != nil {
		return nil, err
	}
	if len(d.data)-d.pos < length {
		return nil, io.ErrUnexpectedEOF
	}
	end := d.pos + length
	value := append([]byte(nil), d.data[d.pos:end]...)
	d.pos = end
	return &Node{Value: value, Start: start, End: end}, nil
}

func (d *decoder) decodeInt(start int) (*Node, error) {
	var buf []byte
	for {
		if d.pos >= len(d.data) {
			return nil, io.ErrUnexpectedEOF
		}
		b := d.readByte()
		if b == 'e' {
			break
		}
		buf = append(buf, b)
	}
	if len(buf) == 0 {
		return nil, errors.New("empty integer")
	}
	if len(buf) > 1 && buf[0] == '0' {
		return nil, errors.New("invalid integer with leading zero")
	}
	if len(buf) > 1 && buf[0] == '-' && buf[1] == '0' {
		return nil, errors.New("invalid negative zero integer")
	}
	value, err := strconv.ParseInt(string(buf), 10, 64)
	if err != nil {
		return nil, err
	}
	return &Node{Value: value, Start: start, End: d.pos}, nil
}

func (d *decoder) decodeList(start int) (*Node, error) {
	var result []interface{}
	var nodes []*Node
	for {
		if d.pos >= len(d.data) {
			return nil, io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			break
		}
		node, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		result = append(result, node.Value)
		nodes = append(nodes, node)
	}

	return &Node{Value: result, Start: start, End: d.pos, List: nodes}, nil
}

func (d *decoder) decodeDict(start int) (*Node, error) {
	result := make(map[string]interface{})
	nodes := make(map[string]*Node)

	for {
		if d.pos >= len(d.data) {
			return nil, io.ErrUnexpectedEOF
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			break
		}
		keyNode, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		keyBytes, ok := keyNode.Value.([]byte)
		if !ok {
			return nil, errors.New("dictionary key must be string")
		}
		valueNode, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		key := string(keyBytes)
		result[key] = valueNode.Value
		nodes[key] = valueNode
	}
	return &Node{Value: result, Start: start, End: d.pos, Dict: nodes}, nil
}

func DecodeBytes(data []byte) (interface{}, error) {
	node, err := DecodeWithRaw(data)
	if err != nil {
		return nil, err
	}
	return node.Value, nil
}

func (d *decoder) readByte() byte {
	b := d.data[d.pos]
	d.pos++
	return b
}
