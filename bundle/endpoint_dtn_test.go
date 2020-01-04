package bundle

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNewDtnEndpoint(t *testing.T) {
	tests := []struct {
		uri   string
		ssp   string
		valid bool
	}{
		{"dtn:none", dtnEndpointDtnNoneSsp, true},
		{"dtn:foo", "foo", true},
		{"dtn://foo/", "//foo/", true},
		{"dtn:", "", false},
		{"dtn", "", false},
		{"DTN:UFF", "", false},
		{"uff:uff", "", false},
		{"", "", false},
	}

	for _, test := range tests {
		ep, err := NewDtnEndpoint(test.uri)

		if err == nil != test.valid {
			t.Fatalf("Expected valid = %t, got err: %v", test.valid, err)
		} else if err == nil {
			if ep.ssp != test.ssp {
				t.Fatalf("Expected SSP %v, got %v", test.ssp, ep.ssp)
			}
		}
	}
}

func TestDtnEndpointCbor(t *testing.T) {
	tests := []struct {
		ep   DtnEndpoint
		data []byte
	}{
		{DtnEndpoint{dtnEndpointDtnNoneSsp}, []byte{0x00}},
		{DtnEndpoint{"foo"}, []byte{0x63, 0x66, 0x6F, 0x6F}},
		{DtnEndpoint{"//foo/"}, []byte{0x66, 0x2F, 0x2F, 0x66, 0x6F, 0x6F, 0x2F}},
	}

	for _, test := range tests {
		var buf bytes.Buffer

		// Marshal
		if err := test.ep.MarshalCbor(&buf); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(buf.Bytes(), test.data) {
			t.Fatalf("Expected %v, got %v", test.data, buf.Bytes())
		}

		// Unmarshal
		var ep DtnEndpoint
		if err := ep.UnmarshalCbor(&buf); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(ep, test.ep) {
			t.Fatalf("Expected %v, got %v", test.ep, ep)
		}
	}
}