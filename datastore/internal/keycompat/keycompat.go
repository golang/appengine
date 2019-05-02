package keycompat

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/golang/protobuf/proto"
)

var (
	// ErrInvalidKey is returned when an invalid key is presented.
	ErrInvalidKey = errors.New("datastore: invalid key")

	ErrKeyConversion = `Key conversions must be enabled in the application.
			See https://github.com/golang/appengine#key-encode-decode-compatibiltiy-to-help-with-datastore-library-migrations for more details.`
)

// This code is duplicated from https://github.com/googleapis/google-cloud-go/blob/master/datastore/key.go with the method renamed.
// decodeToNewKey decodes a key from the opaque representation returned by Encode.
func DecodeToNewKey(encoded string) (*NewFormatKey, error) {
	// Re-add padding.
	if m := len(encoded) % 4; m != 0 {
		encoded += strings.Repeat("=", 4-m)
	}

	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	pKey := new(PBKey)
	if err := proto.Unmarshal(b, pKey); err != nil {
		return nil, err
	}
	return ProtoToKey(pKey)
}

// protoToKey decodes a protocol buffer representation of a key into an
// equivalent *Key object. If the key is invalid, protoToKey will return the
// invalid key along with ErrInvalidKey.
func ProtoToKey(p *PBKey) (*NewFormatKey, error) {
	var key *NewFormatKey
	var namespace string
	if partition := p.PartitionId; partition != nil {
		namespace = partition.NamespaceId
	}
	for _, el := range p.Path {
		key = &NewFormatKey{
			Namespace: namespace,
			Kind:      el.Kind,
			ID:        el.GetId(),
			Name:      el.GetName(),
			Parent:    key,
		}
	}
	if !key.valid() { // Also detects key == nil.
		return key, ErrInvalidKey
	}
	return key, nil
}

// valid returns whether the key is valid.
func (k *NewFormatKey) valid() bool {
	if k == nil {
		return false
	}
	for ; k != nil; k = k.Parent {
		if k.Kind == "" {
			return false
		}
		if k.Name != "" && k.ID != 0 {
			return false
		}
		if k.Parent != nil {
			if k.Parent.Incomplete() {
				return false
			}
			if k.Parent.Namespace != k.Namespace {
				return false
			}
		}
	}
	return true
}

// Incomplete reports whether the key does not refer to a stored entity.
func (k *NewFormatKey) Incomplete() bool {
	return k.Name == "" && k.ID == 0
}
