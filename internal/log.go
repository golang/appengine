// Copyright 2021 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"strings"
)

var logLevelName = map[int64]string{
	0: "DEBUG",
	1: "INFO",
	2: "WARNING",
	3: "ERROR",
	4: "CRITICAL",
}

func logf(c *context, level int64, format string, args ...interface{}) {
	if c == nil {
		panic("not an App Engine context")
	}
	if !IsSecondGen() {
		s := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
		now := timeNow().UTC()
		timestamp := fmt.Sprintf("%d/%02d/%02d %02d:%02d:%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
		fmt.Fprintf(logStream, timestamp+" "+logLevelName[level]+": "+s+"\n")
		return
	}
	var msg string
	if strings.HasPrefix(format, "{") {
		// Assume the message is already structured; leave exactly as-is.
		msg = fmt.Sprintf(format, args...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
	} else {
		// Structure the message to preserve the log levels.
		s := fmt.Sprintf(format, args...)
		msg = fmt.Sprintf(`{"message": %q, "severity": %q}`+"\n", s, logLevelName[level])
	}
	fmt.Fprintf(logStream, msg)
}
