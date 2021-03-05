// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

/*
Package log provides the means of writing an application's logs
from within an App Engine application.

Example:
	c := appengine.NewContext(r)
	if err := doThing(); err != nil {
		log.Errorf(c, "doThing: %v", err)
		return err
	}
	log.Infof(c, "No problems")
*/
package log // import "google.golang.org/appengine/log"

import (
	"time"

	"github.com/golang/protobuf/proto"

	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/log"
)

// AppLog represents a single application-level log.
type AppLog struct {
	Time    time.Time
	Level   int
	Message string
}

// Record contains all the information for a single web request.
type Record struct {
	AppID            string
	ModuleID         string
	VersionID        string
	RequestID        []byte
	IP               string
	Nickname         string
	AppEngineRelease string

	// The time when this request started.
	StartTime time.Time

	// The time when this request finished.
	EndTime time.Time

	// Opaque cursor into the result stream.
	Offset []byte

	// The time required to process the request.
	Latency     time.Duration
	MCycles     int64
	Method      string
	Resource    string
	HTTPVersion string
	Status      int32

	// The size of the request sent back to the client, in bytes.
	ResponseSize int64
	Referrer     string
	UserAgent    string
	URLMapEntry  string
	Combined     string
	Host         string

	// The estimated cost of this request, in dollars.
	Cost              float64
	TaskQueueName     string
	TaskName          string
	WasLoadingRequest bool
	PendingTime       time.Duration
	Finished          bool
	AppLogs           []AppLog

	// Mostly-unique identifier for the instance that handled the request if available.
	InstanceID string
}

// protoToAppLogs takes as input an array of pointers to LogLines, the internal
// Protocol Buffer representation of a single application-level log,
// and converts it to an array of AppLogs, the external representation
// of an application-level log.
func protoToAppLogs(logLines []*pb.LogLine) []AppLog {
	appLogs := make([]AppLog, len(logLines))

	for i, line := range logLines {
		appLogs[i] = AppLog{
			Time:    time.Unix(0, *line.Time*1e3),
			Level:   int(*line.Level),
			Message: *line.LogMessage,
		}
	}

	return appLogs
}

// protoToRecord converts a RequestLog, the internal Protocol Buffer
// representation of a single request-level log, to a Record, its
// corresponding external representation.
func protoToRecord(rl *pb.RequestLog) *Record {
	offset, err := proto.Marshal(rl.Offset)
	if err != nil {
		offset = nil
	}
	return &Record{
		AppID:             *rl.AppId,
		ModuleID:          rl.GetModuleId(),
		VersionID:         *rl.VersionId,
		RequestID:         rl.RequestId,
		Offset:            offset,
		IP:                *rl.Ip,
		Nickname:          rl.GetNickname(),
		AppEngineRelease:  string(rl.GetAppEngineRelease()),
		StartTime:         time.Unix(0, *rl.StartTime*1e3),
		EndTime:           time.Unix(0, *rl.EndTime*1e3),
		Latency:           time.Duration(*rl.Latency) * time.Microsecond,
		MCycles:           *rl.Mcycles,
		Method:            *rl.Method,
		Resource:          *rl.Resource,
		HTTPVersion:       *rl.HttpVersion,
		Status:            *rl.Status,
		ResponseSize:      *rl.ResponseSize,
		Referrer:          rl.GetReferrer(),
		UserAgent:         rl.GetUserAgent(),
		URLMapEntry:       *rl.UrlMapEntry,
		Combined:          *rl.Combined,
		Host:              rl.GetHost(),
		Cost:              rl.GetCost(),
		TaskQueueName:     rl.GetTaskQueueName(),
		TaskName:          rl.GetTaskName(),
		WasLoadingRequest: rl.GetWasLoadingRequest(),
		PendingTime:       time.Duration(rl.GetPendingTime()) * time.Microsecond,
		Finished:          rl.GetFinished(),
		AppLogs:           protoToAppLogs(rl.Line),
		InstanceID:        string(rl.GetCloneKey()),
	}
}

func init() {
	internal.RegisterErrorCodeMap("logservice", pb.LogServiceError_ErrorCode_name)
}
