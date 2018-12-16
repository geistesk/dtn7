package bpa

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ugorji/go/codec"
)

const DTNVersion uint = 7

// PrimaryBlock is a representation of a Primary Bundle Block as defined in
// section 4.2.2.
type PrimaryBlock struct {
	Version            uint
	BundleControlFlags BundleControlFlags
	CRCType            CRCType
	Destination        EndpointID
	SourceNode         EndpointID
	ReportTo           EndpointID
	CreationTimestamp  CreationTimestamp
	Lifetime           uint
	FragmentOffset     uint
	TotalDataLength    uint
	CRC                uint
}

// NewPrimaryBlock creates a new PrimaryBlock with the given parameters. All
// other fields are set to default values.
func NewPrimaryBlock(bundleControlFlags BundleControlFlags,
	destination EndpointID, sourceNode EndpointID,
	creationTimestamp CreationTimestamp, lifetime uint) PrimaryBlock {
	return PrimaryBlock{
		Version:            DTNVersion,
		BundleControlFlags: bundleControlFlags,
		CRCType:            CRCNo,
		Destination:        destination,
		SourceNode:         sourceNode,
		ReportTo:           *DtnNone,
		CreationTimestamp:  creationTimestamp,
		Lifetime:           lifetime,
		FragmentOffset:     0,
		TotalDataLength:    0,
		CRC:                0,
	}
}

// HasFragmentation returns if the Bundle Processing Control Flags indicates a
// fragmented bundle. In this case the FragmentOffset and TotalDataLength fields
// of this struct should become relevant.
func (pb PrimaryBlock) HasFragmentation() bool {
	return pb.BundleControlFlags.Has(BndlCFBundleIsAFragment)
}

// HasCRC retruns if the CRCType indicates a CRC present for this block. In
// this case the CRC field of this struct should become relevant.
func (pb PrimaryBlock) HasCRC() bool {
	return pb.CRCType != CRCNo
}

func (pb PrimaryBlock) CodecEncodeSelf(enc *codec.Encoder) {
	var blockArr = []interface{}{
		pb.Version,
		pb.BundleControlFlags,
		pb.CRCType,
		pb.Destination,
		pb.SourceNode,
		pb.ReportTo,
		pb.CreationTimestamp,
		pb.Lifetime}

	if pb.HasFragmentation() {
		blockArr = append(blockArr, pb.FragmentOffset, pb.TotalDataLength)
	}

	if pb.HasCRC() {
		blockArr = append(blockArr, pb.CRC)
	}

	enc.MustEncode(blockArr)
}

// decodeEndpoints decodes the three defined EndpointIDs. This method is called
// from CodecDecodeSelf.
func (pb *PrimaryBlock) decodeEndpoints(blockArr []interface{}) {
	endpoints := []struct {
		pos     int
		pointer *EndpointID
	}{
		{3, &pb.Destination},
		{4, &pb.SourceNode},
		{5, &pb.ReportTo},
	}

	for _, ep := range endpoints {
		var arr []interface{} = blockArr[ep.pos].([]interface{})

		(*ep.pointer).SchemeName = uint(arr[0].(uint64))
		(*ep.pointer).SchemeSpecificPort = arr[1]

		// The codec library uses uint64 internally but our `dtn:none` is defined
		// by a more generic uint. In case of an `dtn:none` endpoint we have to
		// switch the type.
		if ty := reflect.TypeOf((*ep.pointer).SchemeSpecificPort); ty.Kind() == reflect.Uint64 {
			(*ep.pointer).SchemeSpecificPort = uint((*ep.pointer).SchemeSpecificPort.(uint64))
		}
	}
}

// decodeCreationTimestamp decodes the CreationTimestamp. This method is called
// from CodecDecodeSelf.
func (pb *PrimaryBlock) decodeCreationTimestamp(blockArr []interface{}) {
	for i := 0; i <= 1; i++ {
		pb.CreationTimestamp[i] = uint((blockArr[6].([]interface{}))[i].(uint64))
	}
}

func (pb *PrimaryBlock) CodecDecodeSelf(dec *codec.Decoder) {
	var blockArrPt = new([]interface{})
	dec.MustDecode(blockArrPt)

	var blockArr = *blockArrPt

	if len(blockArr) < 8 || len(blockArr) > 11 {
		panic("blockArr has wrong length (< 8 or > 10)")
	}

	pb.decodeEndpoints(blockArr)
	pb.decodeCreationTimestamp(blockArr)

	pb.Version = uint(blockArr[0].(uint64))
	pb.BundleControlFlags = BundleControlFlags(blockArr[1].(uint64))
	pb.CRCType = CRCType(blockArr[2].(uint64))
	pb.Lifetime = uint(blockArr[7].(uint64))

	switch len(blockArr) {
	case 9:
		// CRC, No Fragmentation
		pb.CRC = uint(blockArr[8].(uint64))

	case 10:
		// No CRC, Fragmentation
		pb.FragmentOffset = uint(blockArr[8].(uint64))
		pb.TotalDataLength = uint(blockArr[9].(uint64))

	case 11:
		// CRC, Fragmentation
		pb.FragmentOffset = uint(blockArr[8].(uint64))
		pb.TotalDataLength = uint(blockArr[9].(uint64))
		pb.CRC = uint(blockArr[10].(uint64))
	}
}

func (pb PrimaryBlock) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "version: %d, ", pb.Version)
	fmt.Fprintf(&b, "bundle processing control flags: %b, ", pb.BundleControlFlags)
	fmt.Fprintf(&b, "crc type: %v, ", pb.CRCType)
	fmt.Fprintf(&b, "destination: %v, ", pb.Destination)
	fmt.Fprintf(&b, "source node: %v, ", pb.SourceNode)
	fmt.Fprintf(&b, "report to: %v, ", pb.ReportTo)
	fmt.Fprintf(&b, "creation timestamp: %v, ", pb.CreationTimestamp)
	fmt.Fprintf(&b, "lifetime: %d", pb.Lifetime)

	if pb.HasFragmentation() {
		fmt.Fprintf(&b, " , ")
		fmt.Fprintf(&b, "fragment offset: %d, ", pb.FragmentOffset)
		fmt.Fprintf(&b, "total data length: %d", pb.TotalDataLength)
	}

	if pb.HasCRC() {
		fmt.Fprintf(&b, " , ")
		fmt.Fprintf(&b, "crc: %x", pb.CRC)
	}

	return b.String()
}