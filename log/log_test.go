// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package log

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pb "google.golang.org/appengine/internal/log"
)

func TestQueryToRequest(t *testing.T) {
	testCases := []struct {
		desc  string
		query *Query
		want  *pb.LogReadRequest
	}{
		{
			desc:  "Empty",
			query: &Query{},
			want: &pb.LogReadRequest{
				AppId:     "s~fake",
				VersionId: []string{"v12"},
			},
		},
		{
			desc: "Versions",
			query: &Query{
				Versions: []string{"alpha", "backend:beta"},
			},
			want: &pb.LogReadRequest{
				AppId: "s~fake",
				ModuleVersion: []*pb.LogModuleVersion{
					{
						VersionId: proto.String("alpha"),
					}, {
						ModuleId:  proto.String("backend"),
						VersionId: proto.String("beta"),
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		req, err := makeRequest(tt.query, "s~fake", "v12")

		if err != nil {
			t.Errorf("%s: got err %v, want nil", tt.desc, err)
			continue
		}
		if !proto.Equal(req, tt.want) {
			t.Errorf("%s request:\ngot  %v\nwant %v", tt.desc, req, tt.want)
		}
	}
}

func TestProtoToRecord(t *testing.T) {
	// We deliberately leave ModuleId and other optional fields unset.
	p := &pb.RequestLog{
		AppId:        "s~fake",
		VersionId:    "1",
		RequestId:    []byte("deadbeef"),
		Ip:           "127.0.0.1",
		StartTime:    431044244000000,
		EndTime:      431044724000000,
		Latency:      480000000,
		Mcycles:      7,
		Method:       "GET",
		Resource:     "/app",
		HttpVersion:  "1.1",
		Status:       418,
		ResponseSize: 1337,
		UrlMapEntry:  "_go_app",
		Combined:     "apache log",
	}
	// Sanity check that all required fields are set.
	if _, err := proto.Marshal(p); err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	want := &Record{
		AppID:        "s~fake",
		ModuleID:     "",
		VersionID:    "1",
		RequestID:    []byte("deadbeef"),
		IP:           "127.0.0.1",
		StartTime:    time.Date(1983, 8, 29, 22, 30, 44, 0, time.UTC),
		EndTime:      time.Date(1983, 8, 29, 22, 38, 44, 0, time.UTC),
		Latency:      8 * time.Minute,
		MCycles:      7,
		Method:       "GET",
		Resource:     "/app",
		HTTPVersion:  "1.1",
		Status:       418,
		ResponseSize: 1337,
		URLMapEntry:  "_go_app",
		Combined:     "apache log",
		Finished:     false,
		AppLogs:      []AppLog{},
	}
	got := protoToRecord(p)
	// Coerce locations to UTC since otherwise they will be in local.
	got.StartTime, got.EndTime = got.StartTime.UTC(), got.EndTime.UTC()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("protoToRecord:\ngot:  %v\nwant: %v", got, want)
	}
}
