package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func NewEventID() (string, error)      { return newTypedID("evt") }
func NewRecordID() (string, error)     { return newTypedID("rec") }
func NewThreadID() (string, error)     { return newTypedID("thr") }
func NewProjectionID() (string, error) { return newTypedID("prj") }
func NewRelationID() (string, error)   { return newTypedID("rel") }
func NewReceiptID() (string, error)    { return newTypedID("rcp") }

func newTypedID(prefix string) (string, error) {
	var id [16]byte
	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return "", err
	}
	millis := uint64(time.Now().UnixMilli())
	id[0] = byte(millis >> 40)
	id[1] = byte(millis >> 32)
	id[2] = byte(millis >> 24)
	id[3] = byte(millis >> 16)
	id[4] = byte(millis >> 8)
	id[5] = byte(millis)
	id[6] = (id[6] & 0x0f) | 0x70
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s_%08x-%04x-%04x-%04x-%012x",
		prefix,
		binary.BigEndian.Uint32(id[0:4]),
		binary.BigEndian.Uint16(id[4:6]),
		binary.BigEndian.Uint16(id[6:8]),
		binary.BigEndian.Uint16(id[8:10]),
		uint64(id[10])<<40|uint64(id[11])<<32|uint64(id[12])<<24|uint64(id[13])<<16|uint64(id[14])<<8|uint64(id[15])), nil
}

func ValidTypedID(value, prefix string) bool {
	value = strings.TrimPrefix(value, prefix+"_")
	return value != "" && uuidV7Pattern.MatchString(value)
}

// DeriveRelationID creates a stable UUIDv7-shaped relation ID using the
// timestamp portion of a receipt ID and a hashed discriminator.
func DeriveRelationID(receiptID, discriminator string) (string, error) {
	if !ValidTypedID(receiptID, "rcp") {
		return "", fmt.Errorf("receipt ID must be an rcp_ UUIDv7")
	}
	compact := strings.ReplaceAll(strings.TrimPrefix(receiptID, "rcp_"), "-", "")
	raw, _ := hex.DecodeString(compact)
	digest := sha256.Sum256([]byte(receiptID + "\x00" + discriminator))
	copy(raw[6:], digest[6:])
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("rel_%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(raw[0:4]),
		binary.BigEndian.Uint16(raw[4:6]),
		binary.BigEndian.Uint16(raw[6:8]),
		binary.BigEndian.Uint16(raw[8:10]),
		uint64(raw[10])<<40|uint64(raw[11])<<32|uint64(raw[12])<<24|uint64(raw[13])<<16|uint64(raw[14])<<8|uint64(raw[15])), nil
}
