package bencode

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
)

func Encode(v interface{}) []byte {

	switch val := v.(type) {

	case string:
		return encodeString([]byte(val))

	case []byte:
		return encodeString(val)

	case int:
		return encodeInt(int64(val))

	case int64:
		return encodeInt(val)

	case []interface{}:
		return encodeList(val)

	case map[string]interface{}:
		return encodeDict(val)

	default:
		panic(fmt.Sprintf("unsupported type for bencode: %T", v))
	}
}

func encodeString(data []byte) []byte {

	var buf bytes.Buffer

	buf.WriteString(strconv.Itoa(len(data)))
	buf.WriteByte(':')
	buf.Write(data)

	return buf.Bytes()
}

func encodeInt(i int64) []byte {
	return []byte(fmt.Sprintf("i%de", i))
}

func encodeList(list []interface{}) []byte {

	var buf bytes.Buffer

	buf.WriteByte('l')

	for _, item := range list {
		buf.Write(Encode(item))
	}

	buf.WriteByte('e')

	return buf.Bytes()
}

func encodeDict(dict map[string]interface{}) []byte {

	var buf bytes.Buffer

	buf.WriteByte('d')

	keys := make([]string, 0, len(dict))

	for k := range dict {
		keys = append(keys, k)
	}

	// REQUIRED by BitTorrent spec
	sort.Strings(keys)

	for _, key := range keys {

		buf.Write(encodeString([]byte(key)))
		buf.Write(Encode(dict[key]))
	}

	buf.WriteByte('e')

	return buf.Bytes()
}
