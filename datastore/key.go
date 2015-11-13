// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/datastore"
)

// Key represents the datastore key for a stored entity, and is immutable.
type Key struct {
	kind      string
	stringID  string
	intID     int64
	parent    *Key
	appID     string
	namespace string
}

// Kind returns the key's kind (also known as entity type).
func (k *Key) Kind() string {
	return k.kind
}

// StringID returns the key's string ID (also known as an entity name or key
// name), which may be "".
func (k *Key) StringID() string {
	return k.stringID
}

// IntID returns the key's integer ID, which may be 0.
func (k *Key) IntID() int64 {
	return k.intID
}

// Parent returns the key's parent key, which may be nil.
func (k *Key) Parent() *Key {
	return k.parent
}

// AppID returns the key's application ID.
func (k *Key) AppID() string {
	return k.appID
}

// Namespace returns the key's namespace.
func (k *Key) Namespace() string {
	return k.namespace
}

// Incomplete returns whether the key does not refer to a stored entity.
// In particular, whether the key has a zero StringID and a zero IntID.
func (k *Key) Incomplete() bool {
	return k.stringID == "" && k.intID == 0
}

// valid returns whether the key is valid.
func (k *Key) valid() bool {
	if k == nil {
		return false
	}
	for ; k != nil; k = k.parent {
		if k.kind == "" || k.appID == "" {
			return false
		}
		if k.stringID != "" && k.intID != 0 {
			return false
		}
		if k.parent != nil {
			if k.parent.Incomplete() {
				return false
			}
			if k.parent.appID != k.appID || k.parent.namespace != k.namespace {
				return false
			}
		}
	}
	return true
}

// Equal returns whether two keys are equal.
func (k *Key) Equal(o *Key) bool {
	for k != nil && o != nil {
		if k.kind != o.kind || k.stringID != o.stringID || k.intID != o.intID || k.appID != o.appID || k.namespace != o.namespace {
			return false
		}
		k, o = k.parent, o.parent
	}
	return k == o
}

// root returns the furthest ancestor of a key, which may be itself.
func (k *Key) root() *Key {
	for k.parent != nil {
		k = k.parent
	}
	return k
}

// marshal marshals the key's string representation to the buffer.
func (k *Key) marshal(b *bytes.Buffer) {
	if k.parent != nil {
		k.parent.marshal(b)
	}
	b.WriteByte('/')
	b.WriteString(k.kind)
	b.WriteByte(',')
	if k.stringID != "" {
		b.WriteString(k.stringID)
	} else {
		b.WriteString(strconv.FormatInt(k.intID, 10))
	}
}

// String returns a string representation of the key.
func (k *Key) String() string {
	if k == nil {
		return ""
	}
	b := bytes.NewBuffer(make([]byte, 0, 512))
	k.marshal(b)
	return b.String()
}

type gobKey struct {
	Kind      string
	StringID  string
	IntID     int64
	Parent    *gobKey
	AppID     string
	Namespace string
}

func keyToGobKey(k *Key) *gobKey {
	if k == nil {
		return nil
	}
	return &gobKey{
		Kind:      k.kind,
		StringID:  k.stringID,
		IntID:     k.intID,
		Parent:    keyToGobKey(k.parent),
		AppID:     k.appID,
		Namespace: k.namespace,
	}
}

func gobKeyToKey(gk *gobKey) *Key {
	if gk == nil {
		return nil
	}
	return &Key{
		kind:      gk.Kind,
		stringID:  gk.StringID,
		intID:     gk.IntID,
		parent:    gobKeyToKey(gk.Parent),
		appID:     gk.AppID,
		namespace: gk.Namespace,
	}
}

func (k *Key) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(keyToGobKey(k)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (k *Key) GobDecode(buf []byte) error {
	gk := new(gobKey)
	if err := gob.NewDecoder(bytes.NewBuffer(buf)).Decode(gk); err != nil {
		return err
	}
	*k = *gobKeyToKey(gk)
	return nil
}

func (k *Key) MarshalJSON() ([]byte, error) {
	return []byte(`"` + k.Encode() + `"`), nil
}

func (k *Key) UnmarshalJSON(buf []byte) error {
	if len(buf) < 2 || buf[0] != '"' || buf[len(buf)-1] != '"' {
		return errors.New("datastore: bad JSON key")
	}
	k2, err := DecodeKey(string(buf[1 : len(buf)-1]))
	if err != nil {
		return err
	}
	*k = *k2
	return nil
}

// Encode returns an opaque representation of the key
// suitable for use in HTML and URLs.
// This is compatible with the Python and Java runtimes.
func (k *Key) Encode() string {
	ref := keyToProto("", k)

	b, err := proto.Marshal(ref)
	if err != nil {
		panic(err)
	}

	// Trailing padding is stripped.
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// DecodeKey decodes a key from the opaque representation returned by Encode.
func DecodeKey(encoded string) (*Key, error) {
	// Re-add padding.
	if m := len(encoded) % 4; m != 0 {
		encoded += strings.Repeat("=", 4-m)
	}

	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	ref := new(pb.Reference)
	if err := proto.Unmarshal(b, ref); err != nil {
		return nil, err
	}

	return protoToKey(ref)
}

// NewIncompleteKey creates a new incomplete key.
// kind cannot be empty.
func NewIncompleteKey(c context.Context, kind string, parent *Key) *Key {
	return NewKey(c, kind, "", 0, parent)
}

// NewKey creates a new key.
// kind cannot be empty.
// Either one or both of stringID and intID must be zero. If both are zero,
// the key returned is incomplete.
// parent must either be a complete key or nil.
func NewKey(c context.Context, kind, stringID string, intID int64, parent *Key) *Key {
	// If there's a parent key, use its namespace.
	// Otherwise, use any namespace attached to the context.
	var namespace string
	if parent != nil {
		namespace = parent.namespace
	} else {
		namespace = internal.NamespaceFromContext(c)
	}

	return &Key{
		kind:      kind,
		stringID:  stringID,
		intID:     intID,
		parent:    parent,
		appID:     internal.FullyQualifiedAppID(c),
		namespace: namespace,
	}
}

// AllocateIDs returns a range of n integer IDs with the given kind and parent
// combination. kind cannot be empty; parent may be nil. The IDs in the range
// returned will not be used by the datastore's automatic ID sequence generator
// and may be used with NewKey without conflict.
//
// The range is inclusive at the low end and exclusive at the high end. In
// other words, valid intIDs x satisfy low <= x && x < high.
//
// If no error is returned, low + n == high.
func AllocateIDs(c context.Context, kind string, parent *Key, n int) (low, high int64, err error) {
	if kind == "" {
		return 0, 0, errors.New("datastore: AllocateIDs given an empty kind")
	}
	if n < 0 {
		return 0, 0, fmt.Errorf("datastore: AllocateIDs given a negative count: %d", n)
	}
	if n == 0 {
		return 0, 0, nil
	}
	req := &pb.AllocateIdsRequest{
		ModelKey: keyToProto("", NewIncompleteKey(c, kind, parent)),
		Size:     proto.Int64(int64(n)),
	}
	res := &pb.AllocateIdsResponse{}
	if err := internal.Call(c, "datastore_v3", "AllocateIds", req, res); err != nil {
		return 0, 0, err
	}
	// The protobuf is inclusive at both ends. Idiomatic Go (e.g. slices, for loops)
	// is inclusive at the low end and exclusive at the high end, so we add 1.
	low = res.GetStart()
	high = res.GetEnd() + 1
	if low+int64(n) != high {
		return 0, 0, fmt.Errorf("datastore: internal error: could not allocate %d IDs", n)
	}
	return low, high, nil
}

type KeyRangeState int

const (
	//	Indicates that an error occurred while determining the KeyRangeState for [start, end] and
	// the returned KeyRangeState should not be trusted. Does not affect high, low
	KeyRangeError KeyRangeState = iota
	// Indicates that entities with keys inside [start, end] already exist and writing to this range
	// will overwrite those entities. Additionally the implications of CONTENTION apply.
	// If overwriting entities that exist in this range is acceptable it is safe to use the given range.
	// The datastore's automatic ID allocator will never assign a key to a new entity that will overwrite an existing entity,
	// so entities written by the user to this range will never be overwritten by an entity with an automatically assigned key.
	KeyRangeCollision
	// Indicates [start, end] is empty but the datastore's automatic ID allocator may assign new entities keys in this range. However it is safe to manually assign Keys in this range if either of the following is true:
	// - No other request will insert entities with the same kind and parent as the given KeyRange until all entities with manually assigned keys from this range have been written.
	// - Overwriting entities written by other requests with the same kind and parent as the given KeyRange is acceptable.
	//
	// The datastore's automatic ID allocator will not assign a key to a new entity that will overwrite an existing entity, so once the range is populated there will no longer be any contention.
	KeyRangeContention
	// Indicates [start, end] is empty and the datastore's automatic ID allocator will not assign keys in this range to new entities.
	KeyRangeEmpty
)

// AllocateIDRange returns a range of integer IDs below end with the given kind and parent
// combination. kind cannot be empty; parent may be nil, start has to be greater than 0,
// end has to be greater or equal to start.
// The IDs in the range [low,high[ returned will not be used by the datastore's automatic ID sequence generator
// and may be used with NewKey without conflict.
//
// The range is inclusive at the low end and exclusive at the high end. In
// other words, valid intIDs x satisfy low <= x && x < high.
//
// Additionally the keyRangeState will indicate the state of IDs in the range of [start, end].
// Possible values are KeyRangeError, KeyRangeCollision, KeyRangeContention and KeyRangeEmpty.
//
// If no error is returned, keyRangeState can be trusted. In any case the IDs in range [low, high[ are safe to use.
func AllocateIDRange(c context.Context, kind string, parent *Key, start int64, end int64) (keyRangeState KeyRangeState, low, high int64, err error) {
	if kind == "" {
		return KeyRangeError, 0, 0, errors.New("datastore: AllocateIdRange given an empty kind")
	}
	if start < 1 {
		return KeyRangeError, 0, 0, fmt.Errorf("datastore: AllocateIdRange Illegal start %d: less than 1", start)
	}
	if end < start {
		return KeyRangeError, 0, 0, fmt.Errorf("datastore: AllocateIdRange Illegal end %d: less than start %d", end, start)
	}
	if parent != nil && parent.Incomplete() {
		return KeyRangeError, 0, 0, errors.New("datastore: AllocateIdRange parent must be complete")
	}
	req := &pb.AllocateIdsRequest{
		ModelKey: keyToProto("", NewIncompleteKey(c, kind, parent)),
		Max:      proto.Int64(end),
	}
	res := &pb.AllocateIdsResponse{}
	if err := internal.Call(c, "datastore_v3", "AllocateIds", req, res); err != nil {
		return KeyRangeError, 0, 0, err
	}
	// The protobuf is inclusive at both ends. Idiomatic Go (e.g. slices, for loops)
	// is inclusive at the low end and exclusive at the high end, so we add 1.
	low = res.GetStart()
	high = res.GetEnd() + 1
	q := NewQuery(kind).KeysOnly().Limit(1)
	q = q.Filter("__key__ >=", NewKey(c, kind, "", start, parent))
	q = q.Filter("__key__ <=", NewKey(c, kind, "", end, parent))
	t := q.Run(c)
	if _, err := t.Next(nil); err == nil {
		return KeyRangeCollision, low, high, nil
	} else if err != Done {
		return KeyRangeError, low, high, err
	}
	// check for race condition
	if start < low {
		return KeyRangeContention, low, high, nil
	} else {
		return KeyRangeEmpty, low, high, nil
	}
}
