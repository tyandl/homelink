package types

import (
	"crypto/rand"
	"fmt"
)

// MessageId is the msgid that correlates a request with its response. It is an alias for
// string so it remains directly assignable to a request packet's string MsgId field.
type MessageId = string

// NewMessageId returns a random RFC 4122 version 4 UUID used as a MessageId.
func NewMessageId() MessageId {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; if it does, send the request without a msgid.
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
