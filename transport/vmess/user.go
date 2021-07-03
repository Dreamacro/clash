package vmess

import (
	"bytes"
	"crypto/md5"

	"github.com/gofrs/uuid"
)

// ID cmdKey length
const (
	IDBytesLen = 16
)

// The ID of en entity, in the form of a UUID.
type ID struct {
	UUID   *uuid.UUID
	CmdKey []byte
}

// newID returns an ID with given UUID.
func newID(uuid *uuid.UUID) *ID {
	id := &ID{UUID: uuid, CmdKey: make([]byte, IDBytesLen)}
	md5hash := md5.New()
	md5hash.Write(uuid.Bytes())
	md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21"))
	md5hash.Sum(id.CmdKey[:0])
	return id
}

func nextID(u *uuid.UUID) *uuid.UUID {
	md5hash := md5.New()
	md5hash.Write(u.Bytes())
	md5hash.Write([]byte("16167dc8-16b6-4e6d-b8bb-65dd68113a81"))
	var newid uuid.UUID
	for {
		md5hash.Sum(newid[:0])
		if !bytes.Equal(newid.Bytes(), u.Bytes()) {
			return &newid
		}
		md5hash.Write([]byte("533eff8a-4113-4b10-b5ce-0f5d76b98cd2"))
	}
}

func newAlterIDs(primary *ID, alterIDCount uint16) []*ID {
	alterIDs := make([]*ID, alterIDCount)
	prevID := primary.UUID
	for idx := range alterIDs {
		newid := nextID(prevID)
		alterIDs[idx] = &ID{UUID: newid, CmdKey: primary.CmdKey[:]}
		prevID = newid
	}
	alterIDs = append(alterIDs, primary)
	return alterIDs
}
