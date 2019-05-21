package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// BundleBuilder is a simple framework to create bundles by method chaining.
//
//   bndl, err := bundle.Builder().
//     CRC(bundle.CRC32).
//     Source("dtn://src/").
//     Destination("dtn://dest/").
//     CreationTimestampNow().
//     Lifetime("30m").
//     HopCountBlock(64).
//     PayloadBlock("hello world!").
//     Build()
//
type BundleBuilder struct {
	err error

	primary          PrimaryBlock
	canonicals       []CanonicalBlock
	canonicalCounter uint
	crcType          CRCType
}

// Builder creates a new BundleBuilder.
func Builder() *BundleBuilder {
	return &BundleBuilder{
		err: nil,

		primary:          PrimaryBlock{Version: dtnVersion},
		canonicals:       []CanonicalBlock{},
		canonicalCounter: 1,
		crcType:          CRCNo,
	}
}

// Error returns the BundleBuilder's error, if one is present.
func (bldr *BundleBuilder) Error() error {
	return bldr.err
}

// CRC sets the bundle's CRC value.
func (bldr *BundleBuilder) CRC(crcType CRCType) *BundleBuilder {
	if bldr.err == nil {
		bldr.crcType = crcType
	}

	return bldr
}

// Build creates a new Bundle and returns an optional error.
func (bldr *BundleBuilder) Build() (bndl Bundle, err error) {
	if bldr.err != nil {
		err = bldr.err
		return
	}

	// Set ReportTo to Source, if it was not set before
	if bldr.primary.ReportTo == (EndpointID{}) {
		bldr.primary.ReportTo = bldr.primary.SourceNode
	}

	// Source and Destination are necessary
	if bldr.primary.SourceNode == (EndpointID{}) || bldr.primary.Destination == (EndpointID{}) {
		err = fmt.Errorf("Both Source and Destination must be set")
		return
	}

	// TODO: sort canonicals

	bndl, err = NewBundle(bldr.primary, bldr.canonicals)
	if err == nil {
		bndl.SetCRCType(bldr.crcType)
		bndl.CalculateCRC()
	}

	return
}

// Helper functions

// bldrParseEndpoint returns an EndpointID for a given EndpointID or a string,
// representing an endpoint identifier as an URI.
func bldrParseEndpoint(eid interface{}) (e EndpointID, err error) {
	switch eid.(type) {
	case EndpointID:
		e = eid.(EndpointID)
	case string:
		e, err = NewEndpointID(eid.(string))
	default:
		err = fmt.Errorf("%T is neither an EndpointID nor a string", eid)
	}
	return
}

// bldrParseLifetime returns a microsecond as an uint for a given microsecond
// or a duration string, which will be parsed.
func bldrParseLifetime(duration interface{}) (us uint, err error) {
	switch duration.(type) {
	case uint:
		us = duration.(uint)
	case int:
		if duration.(int) < 0 {
			err = fmt.Errorf("Lifetime's duratoin %d <= 0", duration.(int))
		} else {
			us = uint(duration.(int))
		}
	case string:
		dur, durErr := time.ParseDuration(duration.(string))
		if durErr != nil {
			err = durErr
		} else if dur <= 0 {
			err = fmt.Errorf("Lifetime's duration %d <= 0", dur)
		} else {
			us = uint(dur.Nanoseconds() / 1000)
		}
	default:
		err = fmt.Errorf(
			"%T is neither an uin nor a string for a Duration", duration)
	}
	return
}

// PrimaryBlock related methods

// Destination sets the bundle's destination, stored in its primary block.
func (bldr *BundleBuilder) Destination(eid interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	if e, err := bldrParseEndpoint(eid); err != nil {
		bldr.err = err
	} else {
		bldr.primary.Destination = e
	}

	return bldr
}

// Source sets the bundle's source, stored in its primary block.
func (bldr *BundleBuilder) Source(eid interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	if e, err := bldrParseEndpoint(eid); err != nil {
		bldr.err = err
	} else {
		bldr.primary.SourceNode = e
	}

	return bldr
}

// ReportTo sets the bundle's report-to address, stored in its primary block.
func (bldr *BundleBuilder) ReportTo(eid interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	if e, err := bldrParseEndpoint(eid); err != nil {
		bldr.err = err
	} else {
		bldr.primary.ReportTo = e
	}

	return bldr
}

// creationTimestamp sets the bundle's creation timestamp.
func (bldr *BundleBuilder) creationTimestamp(t DtnTime) *BundleBuilder {
	if bldr.err == nil {
		bldr.primary.CreationTimestamp = NewCreationTimestamp(t, 0)
	}

	return bldr
}

// CreationTimestampEpoch sets the bundle's creation timestamp to the epoch
// time, stored in its primary block.
func (bldr *BundleBuilder) CreationTimestampEpoch() *BundleBuilder {
	return bldr.creationTimestamp(DtnTimeEpoch)
}

// CreationTimestampNow sets the bundle's creation timestamp to the current
// time, stored in its primary block.
func (bldr *BundleBuilder) CreationTimestampNow() *BundleBuilder {
	return bldr.creationTimestamp(DtnTimeNow())
}

// CreationTimestampTime sets the bundle's creation timestamp to a given time,
// stored in its primary block.
func (bldr *BundleBuilder) CreationTimestampTime(t time.Time) *BundleBuilder {
	return bldr.creationTimestamp(DtnTimeFromTime(t))
}

// Lifetime sets the bundle's lifetime, stored in its primary block. Possible
// values are an uint/int, representing the lifetime in microseconds or a format
// string for the duration. This string is passed to time.ParseDuration.
//
//   Lifetime(1000)     // Lifetime of 1000us
//   Lifetime("1000us") // Lifetime of 1000us
//   Lifetime("10m")    // Lifetime of 10min
//
func (bldr *BundleBuilder) Lifetime(duration interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	if us, usErr := bldrParseLifetime(duration); usErr != nil {
		bldr.err = usErr
	} else {
		bldr.primary.Lifetime = us
	}

	return bldr
}

// BundleBuilder sets the bundle processing controll flags in the primary block.
func (bldr *BundleBuilder) BundleCtrlFlags(bcf BundleControlFlags) *BundleBuilder {
	if bldr.err == nil {
		bldr.primary.BundleControlFlags = bcf
	}

	return bldr
}

// CanonicalBlock related methods

// Canonical adds a canonical block to this bundle. The parameters are:
//
//   BlockType, Data[, BlockControlFlags]
//
//   where BlockType is a bundle.CanonicalBlockType,
//   Data is the block's data in its specific type and
//   BlockControlFlags are _optional_ block processing controll flags
//
func (bldr *BundleBuilder) Canonical(args ...interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	var (
		blockNumber    uint
		blockType      CanonicalBlockType
		data           interface{}
		blockCtrlFlags BlockControlFlags

		chk0, chk1 bool = true, true
	)

	switch l := len(args); l {
	case 2:
		blockType, chk0 = args[0].(CanonicalBlockType)
		data = args[1]
	case 3:
		blockType, chk0 = args[0].(CanonicalBlockType)
		data = args[1]
		blockCtrlFlags, chk1 = args[2].(BlockControlFlags)
	default:
		bldr.err = fmt.Errorf(
			"Canonical was called with neither two nor three parameters")
		return bldr
	}

	if !(chk0 && chk1) {
		bldr.err = fmt.Errorf("Canonical received wrong parameter types, %v %v", chk0, chk1)
		return bldr
	}

	if blockType == PayloadBlock {
		blockNumber = 0
	} else {
		blockNumber = bldr.canonicalCounter
		bldr.canonicalCounter++
	}

	bldr.canonicals = append(bldr.canonicals,
		NewCanonicalBlock(blockType, blockNumber, blockCtrlFlags, data))

	return bldr
}

// BundleAgeBlock adds a bundle age block to this bundle. The parameters are:
//
//   Age[, BlockControlFlags]
//
//   where Age is the age as an uint in microsecond or a format string and
//   BlockControlFlags are _optional_ block processing controll flags
//
func (bldr *BundleBuilder) BundleAgeBlock(args ...interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	us, usErr := bldrParseLifetime(args[0])
	if usErr != nil {
		bldr.err = usErr
	}

	// Call Canonical as a variadic function with:
	// - BlockType: BundleAgeBlock,
	// - Data: us (us parsed from given age)
	// - BlockControlFlags: BlockControlFlags, if given
	return bldr.Canonical(
		append([]interface{}{BundleAgeBlock, us}, args[1:]...)...)
}

// HopCountBlock adds a hop count block to this bundle. The parameters are:
//
//   Limit[, BlockControlFlags]
//
//   where Limit is the limit of this Hop Count Block and
//   BlockControlFlags are _optional_ block processing controll flags
//
func (bldr *BundleBuilder) HopCountBlock(args ...interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	limit, chk := args[0].(int)
	if !chk {
		bldr.err = fmt.Errorf("HopCountBlock received wrong parameter type")
	}

	// Read the comment in BundleAgeBlock to grasp the following madness
	return bldr.Canonical(append(
		[]interface{}{HopCountBlock, NewHopCount(uint(limit))}, args[1:]...)...)
}

// PayloadBlock adds a payload block to this bundle. The parameters are:
//
//   Data[, BlockControlFlags]
//
//   where Data is the payload's data and
//   BlockControlFlags are _optional_ block processing controll flags
func (bldr *BundleBuilder) PayloadBlock(args ...interface{}) *BundleBuilder {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, args[0]); err != nil {
		bldr.err = err
		return bldr
	}

	// Call Canonical, but add PayloadBlock as the first variadic parameter
	return bldr.Canonical(append(
		[]interface{}{PayloadBlock, []byte(buf.Bytes())}, args[1:]...)...)
}

// PreviousNodeBlock adds a previous node block to this bundle. The parameters
// are:
//
//   PrevNode[, BlockControlFlags]
//
//   where PrevNode is an EndpointID or a string describing an endpoint and
//   BlockControlFlags are _optional_ block processing controll flags
//
func (bldr *BundleBuilder) PreviousNodeBlock(args ...interface{}) *BundleBuilder {
	if bldr.err != nil {
		return bldr
	}

	eid, eidErr := bldrParseEndpoint(args[0])
	if eidErr != nil {
		bldr.err = eidErr
	}

	return bldr.Canonical(
		append([]interface{}{PreviousNodeBlock, eid}, args[1:]...)...)
}