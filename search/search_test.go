// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package search

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"code.google.com/p/goprotobuf/proto"

	"google.golang.org/appengine"
	pb "google.golang.org/appengine/internal/search"
)

type TestDoc struct {
	String   string
	Atom     Atom
	HTML     HTML
	Float    float64
	Location appengine.GeoPoint
	Time     time.Time
}

var (
	float       = 3.14159
	floatOut    = "3.14159e+00"
	latitude    = 37.3894
	longitude   = 122.0819
	testGeo     = appengine.GeoPoint{latitude, longitude}
	testString  = "foo<b>bar"
	testTime    = time.Unix(1337324400, 0)
	testTimeOut = "1337324400000"
	searchDoc   = TestDoc{
		testString,
		Atom(testString),
		HTML(testString),
		float,
		testGeo,
		testTime,
	}
	searchFields = FieldList{
		Field{"String", testString},
		Field{"Atom", Atom(testString)},
		Field{"HTML", HTML(testString)},
		Field{"Float", float},
		Field{"Location", testGeo},
		Field{"Time", testTime},
	}
	protoFields = []*pb.Field{
		newStringValueField("String", testString, pb.FieldValue_TEXT),
		newStringValueField("Atom", testString, pb.FieldValue_ATOM),
		newStringValueField("HTML", testString, pb.FieldValue_HTML),
		newStringValueField("Float", floatOut, pb.FieldValue_NUMBER),
		&pb.Field{
			Name: proto.String("Location"),
			Value: &pb.FieldValue{
				Geo: &pb.FieldValue_Geo{
					Lat: proto.Float64(latitude),
					Lng: proto.Float64(longitude),
				},
				Type: pb.FieldValue_GEO.Enum(),
			},
		},
		newStringValueField("Time", testTimeOut, pb.FieldValue_DATE),
	}
)

func newStringValueField(name string, value string, valueType pb.FieldValue_ContentType) *pb.Field {
	return &pb.Field{
		Name: proto.String(name),
		Value: &pb.FieldValue{
			StringValue: proto.String(value),
			Type:        valueType.Enum(),
		},
	}
}

func TestValidIndexNameOrDocID(t *testing.T) {
	testCases := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"!", false},
		{"$", true},
		{"!bad", false},
		{"good!", true},
		{"alsoGood", true},
		{"has spaces", false},
		{"is_inva\xffid_UTF-8", false},
		{"is_non-ASCïI", false},
		{"underscores_are_ok", true},
	}
	for _, tc := range testCases {
		if got := validIndexNameOrDocID(tc.s); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestLoadFields(t *testing.T) {
	got, want := TestDoc{}, searchDoc
	err := loadFields(&got, protoFields)
	if err != nil {
		t.Fatalf("loadFields: %v", err)
	}
	if got != want {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}
}

func TestSaveFields(t *testing.T) {
	got, err := saveFields(&searchDoc)
	if err != nil {
		t.Fatalf("saveFields: %v", err)
	}
	want := protoFields
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}
}

func TestLoadFieldList(t *testing.T) {
	got, want := FieldList{}, searchFields
	err := loadFields(&got, protoFields)
	if err != nil {
		t.Fatalf("loadFields: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}
}

func TestSaveFieldList(t *testing.T) {
	got, err := saveFields(&searchFields)
	if err != nil {
		t.Fatalf("saveFields: %v", err)
	}
	want := protoFields
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\ngot  %v\nwant %v", got, want)
	}
}

func TestValidFieldNames(t *testing.T) {
	testCases := []struct {
		name  string
		valid bool
	}{
		{"Normal", true},
		{"Also_OK_123", true},
		{"Not so great", false},
		{"lower_case", false},
		{"Exclaim!", false},
		{"Hello세상아 안녕", false},
		{"", false},
		{"Hεllo", false},
		{strings.Repeat("A", 500), true},
		{strings.Repeat("A", 501), false},
	}

	for _, tc := range testCases {
		_, err := saveFields(&FieldList{
			Field{Name: tc.name, Value: "val"},
		})
		if err != nil && !strings.Contains(err.Error(), "invalid field name") {
			t.Errorf("unexpected err %q for field name %q", err, tc.name)
		}
		if (err == nil) != tc.valid {
			t.Errorf("field %q: expected valid %t, received err %v", tc.name, tc.valid, err)
		}
	}
}

func TestDuplicateFields(t *testing.T) {
	testCases := []struct {
		desc   string
		fields FieldList
		errMsg string // Non-empty if we expect an error
	}{
		{
			desc:   "multi string",
			fields: FieldList{{"FieldA", "val1"}, {"FieldA", "val2"}, {"FieldA", "val3"}},
		},
		{
			desc:   "multi atom",
			fields: FieldList{{"FieldA", Atom("val1")}, {"FieldA", Atom("val2")}, {"FieldA", Atom("val3")}},
		},
		{
			desc:   "mixed",
			fields: FieldList{{"FieldA", testString}, {"FieldA", testTime}, {"FieldA", float}},
		},
		{
			desc:   "multi time",
			fields: FieldList{{"FieldA", testTime}, {"FieldA", testTime}},
			errMsg: `duplicate time field "FieldA"`,
		},
		{
			desc:   "multi num",
			fields: FieldList{{"FieldA", float}, {"FieldA", float}},
			errMsg: `duplicate numeric field "FieldA"`,
		},
	}
	for _, tc := range testCases {
		_, err := saveFields(&tc.fields)
		if (err == nil) != (tc.errMsg == "") || (err != nil && !strings.Contains(err.Error(), tc.errMsg)) {
			t.Errorf("%s: got err %v, wanted %q", tc.desc, err, tc.errMsg)
		}
	}
}

func TestLoadErrFieldMismatch(t *testing.T) {
	testCases := []struct {
		desc string
		dst  interface{}
		src  []*pb.Field
		err  error
	}{
		{
			desc: "missing",
			dst:  &struct{ One string }{},
			src:  []*pb.Field{newStringValueField("Two", "woop!", pb.FieldValue_TEXT)},
			err: &ErrFieldMismatch{
				FieldName: "Two",
				Reason:    "no such struct field",
			},
		},
		{
			desc: "wrong type",
			dst:  &struct{ Num float64 }{},
			src:  []*pb.Field{newStringValueField("Num", "woop!", pb.FieldValue_TEXT)},
			err: &ErrFieldMismatch{
				FieldName: "Num",
				Reason:    "type mismatch: float64 for string data",
			},
		},
		{
			desc: "unsettable",
			dst:  &struct{ lower string }{},
			src:  []*pb.Field{newStringValueField("lower", "woop!", pb.FieldValue_TEXT)},
			err: &ErrFieldMismatch{
				FieldName: "lower",
				Reason:    "cannot set struct field",
			},
		},
	}
	for _, tc := range testCases {
		err := loadFields(tc.dst, tc.src)
		if !reflect.DeepEqual(err, tc.err) {
			t.Errorf("%s, got err %v, wanted %v", tc.desc, err, tc.err)
		}
	}
}

func TestLimit(t *testing.T) {
	more := func(it *Iterator) error {
		if it.limit == 0 {
			return errors.New("Iterator.limit should not be zero in next")
		}
		// Page up to 20 items at once.
		ret := 20
		if it.limit > 0 && it.limit < ret {
			ret = it.limit
		}
		it.listRes = make([]*pb.Document, ret)
		for i := range it.listRes {
			it.listRes[i] = &pb.Document{}
		}
		return nil
	}

	it := &Iterator{
		more:  more,
		limit: 42,
	}

	count := 0
	for {
		_, err := it.Next(nil)
		if err == Done {
			break
		}
		if err != nil {
			t.Fatalf("err after %d: %v", count, err)
		}
		count++
	}
	if count != 42 {
		t.Errorf("got %d results, expected 42", count)
	}
}
