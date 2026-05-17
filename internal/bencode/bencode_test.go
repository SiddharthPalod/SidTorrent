package bencode

import (
	"bytes"
	"testing"
)

func TestDecodePrimitives(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want interface{}
	}{
		{name: "string", in: []byte("4:spam"), want: []byte("spam")},
		{name: "integer", in: []byte("i42e"), want: int64(42)},
		{name: "negative integer", in: []byte("i-42e"), want: int64(-42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeBytes(tt.in)
			if err != nil {
				t.Fatalf("DecodeBytes() error = %v", err)
			}
			switch want := tt.want.(type) {
			case []byte:
				gotBytes, ok := got.([]byte)
				if !ok {
					t.Fatalf("DecodeBytes() = %T, want []byte", got)
				}
				if !bytes.Equal(gotBytes, want) {
					t.Fatalf("DecodeBytes() = %q, want %q", gotBytes, want)
				}
			default:
				if got != want {
					t.Fatalf("DecodeBytes() = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestDecodePreservesBinaryStrings(t *testing.T) {
	got, err := DecodeBytes([]byte("5:\x00\xffabc"))
	if err != nil {
		t.Fatalf("DecodeBytes() error = %v", err)
	}

	gotBytes, ok := got.([]byte)
	if !ok {
		t.Fatalf("DecodeBytes() = %T, want []byte", got)
	}
	want := []byte{0x00, 0xff, 'a', 'b', 'c'}
	if !bytes.Equal(gotBytes, want) {
		t.Fatalf("DecodeBytes() = %v, want %v", gotBytes, want)
	}
}

func TestDecodeDictKeysAreStrings(t *testing.T) {
	got, err := DecodeBytes([]byte("d3:cow3:moo4:spam4:eggse"))
	if err != nil {
		t.Fatalf("DecodeBytes() error = %v", err)
	}

	dict, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("DecodeBytes() = %T, want map[string]interface{}", got)
	}
	if string(dict["cow"].([]byte)) != "moo" {
		t.Fatalf("cow = %q, want moo", dict["cow"])
	}
	if string(dict["spam"].([]byte)) != "eggs" {
		t.Fatalf("spam = %q, want eggs", dict["spam"])
	}
}

func TestEncodeDecodedValue(t *testing.T) {
	in := []byte("d3:cow3:moo4:spaml1:a1:bee")
	decoded, err := DecodeBytes(in)
	if err != nil {
		t.Fatalf("DecodeBytes() error = %v", err)
	}

	if got := Encode(decoded); !bytes.Equal(got, in) {
		t.Fatalf("Encode(DecodeBytes()) = %q, want %q", got, in)
	}
}

func TestDecodeWithRawTracksDictionaryValueOffsets(t *testing.T) {
	in := []byte("d4:infod3:cow3:mooee")
	node, err := DecodeWithRaw(in)
	if err != nil {
		t.Fatalf("DecodeWithRaw() error = %v", err)
	}

	info := node.Dict["info"]
	if info == nil {
		t.Fatal("info node missing")
	}
	if got := in[info.Start:info.End]; !bytes.Equal(got, []byte("d3:cow3:mooe")) {
		t.Fatalf("raw info = %q, want %q", got, "d3:cow3:mooe")
	}

	dict, ok := info.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("info.Value = %T, want map[string]interface{}", info.Value)
	}
	if string(dict["cow"].([]byte)) != "moo" {
		t.Fatalf("cow = %q, want moo", dict["cow"])
	}
}

func TestDecodeRejectsMalformedValues(t *testing.T) {
	tests := [][]byte{
		[]byte("03:abc"),
		[]byte("i03e"),
		[]byte("i-0e"),
		[]byte("1:a1:b"),
	}

	for _, tt := range tests {
		t.Run(string(tt), func(t *testing.T) {
			if _, err := DecodeBytes(tt); err == nil {
				t.Fatalf("DecodeBytes(%q) succeeded, want error", tt)
			}
		})
	}
}
