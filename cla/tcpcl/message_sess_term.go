package tcpcl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// SessionTerminationFlags are single-bit flags used in the SessionTerminationMessage.
type SessionTerminationFlags uint8

const (
	// TerminationReply indicates that this message is an acknowledgement of an
	// earlier SESS_TERM message.
	TerminationReply SessionTerminationFlags = 0x01
)

func (stf SessionTerminationFlags) String() string {
	var flags []string

	if stf&TerminationReply != 0 {
		flags = append(flags, "REPLY")
	}

	return strings.Join(flags, ",")
}

// SessionTerminationCode is the one-octet refusal reason code for a SESS_TERM message.
type SessionTerminationCode uint8

const (
	// TerminationUnknown indicates an unknown or not specified reason.
	TerminationUnknown SessionTerminationCode = 0x00

	// TerminationIdleTimeout indicates a session being closed due to idleness.
	TerminationIdleTimeout SessionTerminationCode = 0x01

	// TerminationVersionMismatch indicates that the node cannot conform to the
	// specified TCPCL protocol version number.
	TerminationVersionMismatch SessionTerminationCode = 0x02

	// TerminationBusy indicates a too busy node.
	TerminationBusy SessionTerminationCode = 0x03

	// TerminationContactFailure indicates that the node cannot interpret or
	// negotiate contact header options.
	TerminationContactFailure SessionTerminationCode = 0x04

	// TerminationResourceExhaustion indicates that the node has run into some
	// resource limit.
	TerminationResourceExhaustion SessionTerminationCode = 0x05
)

func (stc SessionTerminationCode) String() string {
	switch stc {
	case TerminationUnknown:
		return "Unknown"
	case TerminationIdleTimeout:
		return "Idle timeout"
	case TerminationVersionMismatch:
		return "Version mismatch"
	case TerminationBusy:
		return "Busy"
	case TerminationContactFailure:
		return "Contact Failure"
	case TerminationResourceExhaustion:
		return "Resource Exhaustion"
	default:
		return "INVALID"
	}
}

// SESS_TERM is the Message Header code for a Session Termination Message.
const SESS_TERM uint8 = 0x05

// SessionTerminationMessage is the SESS_TERM message for session termination.
type SessionTerminationMessage struct {
	Flags      SessionTerminationFlags
	ReasonCode SessionTerminationCode
}

// NewSessionTerminationMessage creates a new SessionTerminationMessage with given fields.
func NewSessionTerminationMessage(flags SessionTerminationFlags, reason SessionTerminationCode) SessionTerminationMessage {
	return SessionTerminationMessage{
		Flags:      flags,
		ReasonCode: reason,
	}
}

func (stm SessionTerminationMessage) String() string {
	return fmt.Sprintf(
		"SESS_TERM(Message Flags=%v, Reason Code=%v)",
		stm.Flags, stm.ReasonCode)
}

// MarshalBinary encodes this SessionTerminationMessage into its binary form.
func (stm SessionTerminationMessage) MarshalBinary() (data []byte, err error) {
	var buf = new(bytes.Buffer)
	var fields = []interface{}{SESS_TERM, stm.Flags, stm.ReasonCode}

	for _, field := range fields {
		if binErr := binary.Write(buf, binary.BigEndian, field); binErr != nil {
			err = binErr
			return
		}
	}

	data = buf.Bytes()
	return
}

// UnmarshalBinary decodes a SessionTerminationMessage from its binary form.
func (stm *SessionTerminationMessage) UnmarshalBinary(data []byte) error {
	var buf = bytes.NewReader(data)

	var messageHeader uint8
	if err := binary.Read(buf, binary.BigEndian, &messageHeader); err != nil {
		return err
	} else if messageHeader != SESS_TERM {
		return fmt.Errorf("SESS_TERM's Message Header is wrong: %d instead of %d", messageHeader, SESS_TERM)
	}

	var fields = []interface{}{&stm.Flags, &stm.ReasonCode}

	for _, field := range fields {
		if err := binary.Read(buf, binary.BigEndian, field); err != nil {
			return err
		}
	}

	return nil
}