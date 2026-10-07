package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/datastore"
	"google.golang.org/appengine/v2/internal"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

const (
	pendingTaskKind = "_AE_PendingCloudTask"

	statusPending    = "PENDING"
	statusProcessing = "PROCESSING"
	statusFailed     = "FAILED"

	lockDuration        = 60 * time.Second
	fastPathGracePeriod = 60 * time.Second
	maxSweeperRetries   = 5
	maxLastErrorLength  = 500
	// failedTaskRetention is how long entities that exhausted their retries
	// are kept for inspection before the sweeper deletes them.
	failedTaskRetention = 7 * 24 * time.Hour
)

// PendingCloudTask is a transactional task staged in Datastore. The kind and
// property names are shared with the Java and Python App Engine SDKs, so a
// sweeper in a service written in any of them can dispatch the task.
type PendingCloudTask struct {
	QueueName        string    `datastore:"queue_name"`
	CloudTaskName    string    `datastore:"cloud_task_name"`
	CloudTaskPayload string    `datastore:"cloud_task_payload,noindex"`
	Created          time.Time `datastore:"created"`
	Status           string    `datastore:"status"`
	LockExpires      time.Time `datastore:"lock_expires"`
	RetryCount       int64     `datastore:"retry_count"`
	LastError        string    `datastore:"last_error,noindex"`
	HandledBySweeper bool      `datastore:"handled_by_sweeper"`
	FailedAt         time.Time `datastore:"failed_at"`
	SdkLang          string    `datastore:"sdk_lang"`
}

// sweepBatchSize is the maximum number of entities of each status read by one
// sweep. Later cron runs pick up the rest of a backlog. It is a variable so
// that tests can lower it.
var sweepBatchSize = 500

var (
	pendingTasksMu sync.Mutex
	pendingTasks   = make(map[uint64][]string) // transaction handle -> list of urlsafe keys
)

func init() {
	// Staged tasks are dispatched before RunInTransaction returns, while the
	// request that committed the transaction is still active. App Engine API
	// calls made after the request ends fail because its API ticket is no
	// longer valid.
	internal.PostCommitHook = func(ctx context.Context, handle uint64) {
		dispatchPendingTasks(ctx, handle)
	}
	internal.RollbackHook = func(handle uint64) {
		cleanupPendingTasks(handle)
	}
	http.HandleFunc("/_ah/cloudtask/sweep", handleSweep)
}

// encodeTaskPayload returns the staged form of a task: the JSON object
// {"task": T}, where T is the proto3 JSON form of the Cloud Tasks v2 Task.
// The Java and Python SDKs use the same form.
func encodeTaskPayload(task *taskspb.Task) (string, error) {
	b, err := protojson.Marshal(&taskspb.CreateTaskRequest{Task: task})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeTaskPayload parses a payload written by encodeTaskPayload or by
// another SDK. A payload that is not a JSON object is a binary Task proto
// staged by an earlier version of this package. A binary Task never starts
// with '{', which would be the tag of field 15 with the group wire type.
func decodeTaskPayload(payload string) (*taskspb.Task, error) {
	task := &taskspb.Task{}
	if !strings.HasPrefix(payload, "{") {
		if err := proto.Unmarshal([]byte(payload), task); err != nil {
			return nil, fmt.Errorf("failed to unmarshal staged task: %v", err)
		}
		return task, nil
	}
	var wrapper struct {
		Task json.RawMessage `json:"task"`
	}
	if err := json.Unmarshal([]byte(payload), &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse staged task: %v", err)
	}
	raw := []byte(payload)
	if len(wrapper.Task) > 0 {
		raw = wrapper.Task
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, task); err != nil {
		return nil, fmt.Errorf("failed to parse staged task: %v", err)
	}
	return task, nil
}

// ignoreFieldMismatch treats properties written by another SDK that
// PendingCloudTask does not declare as non-fatal.
func ignoreFieldMismatch(err error) error {
	if _, ok := err.(*datastore.ErrFieldMismatch); ok {
		return nil
	}
	if me, ok := err.(appengine.MultiError); ok {
		anyOther := false
		for i, e := range me {
			if _, isFM := e.(*datastore.ErrFieldMismatch); isFM {
				me[i] = nil
			} else if e != nil {
				anyOther = true
			}
		}
		if !anyOther {
			return nil
		}
		return me
	}
	return err
}

func cleanupPendingTasks(handle uint64) {
	pendingTasksMu.Lock()
	delete(pendingTasks, handle)
	pendingTasksMu.Unlock()
}

func logErrorf(ctx context.Context, format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}

func dispatchPendingTasks(ctx context.Context, handle uint64) {
	pendingTasksMu.Lock()
	urlsafeKeys, ok := pendingTasks[handle]
	if ok {
		delete(pendingTasks, handle)
	}
	pendingTasksMu.Unlock()

	if !ok || len(urlsafeKeys) == 0 {
		return
	}

	keys := make([]*datastore.Key, 0, len(urlsafeKeys))
	for _, urlsafeKey := range urlsafeKeys {
		key, err := datastore.DecodeKey(urlsafeKey)
		if err != nil {
			logErrorf(ctx, "Failed to decode pending task key: %v", err)
			continue
		}
		keys = append(keys, key)
	}
	dispatchKeys(ctx, keys, false)
}

// isDispatchable reports whether a staged task may be (re)dispatched now.
func isDispatchable(task *PendingCloudTask, now time.Time) bool {
	switch task.Status {
	case statusPending, "":
		return true
	case statusProcessing:
		// A dispatcher holds the lock until it expires. A missing expiry means
		// the lock was just taken.
		return !task.LockExpires.IsZero() && !now.Before(task.LockExpires)
	}
	return false
}

// acquireDispatchLock atomically marks a staged task as PROCESSING. It
// returns false if the entity no longer exists or another dispatcher holds
// it.
func acquireDispatchLock(ctx context.Context, key *datastore.Key, handledBySweeper bool) (*PendingCloudTask, bool, error) {
	var task PendingCloudTask
	acquired := false
	err := datastore.RunInTransaction(ctx, func(tc context.Context) error {
		acquired = false
		task = PendingCloudTask{}
		if err := ignoreFieldMismatch(datastore.Get(tc, key, &task)); err != nil {
			if err == datastore.ErrNoSuchEntity {
				return nil // already dispatched and deleted by another dispatcher
			}
			return err
		}
		now := time.Now()
		if !isDispatchable(&task, now) {
			return nil
		}
		task.Status = statusProcessing
		task.LockExpires = now.Add(lockDuration)
		task.HandledBySweeper = handledBySweeper
		if _, err := datastore.Put(tc, key, &task); err != nil {
			return err
		}
		acquired = true
		return nil
	}, nil)
	if err == datastore.ErrConcurrentTransaction {
		return nil, false, nil // another dispatcher is updating this entity
	}
	if err != nil {
		return nil, false, err
	}
	return &task, acquired, nil
}

// recordDispatchFailure atomically records a failed dispatch attempt. It does
// nothing if the entity was deleted meanwhile, so a dispatcher that failed
// never recreates an entity another dispatcher already completed.
func recordDispatchFailure(ctx context.Context, key *datastore.Key, dispatchErr error) error {
	return datastore.RunInTransaction(ctx, func(tc context.Context) error {
		var task PendingCloudTask
		if err := ignoreFieldMismatch(datastore.Get(tc, key, &task)); err != nil {
			if err == datastore.ErrNoSuchEntity {
				return nil
			}
			return err
		}
		task.RetryCount++
		task.LastError = dispatchErr.Error()
		if len(task.LastError) > maxLastErrorLength {
			task.LastError = task.LastError[:maxLastErrorLength]
		}
		task.LockExpires = time.Time{}
		if task.RetryCount >= maxSweeperRetries {
			task.Status = statusFailed
			task.FailedAt = time.Now()
		} else {
			task.Status = statusPending
		}
		_, err := datastore.Put(tc, key, &task)
		return err
	}, nil)
}

// dispatchKeys sends the staged tasks to Cloud Tasks. ctx must not be in a
// transaction.
func dispatchKeys(ctx context.Context, keys []*datastore.Key, handledBySweeper bool) int {
	type lockedItem struct {
		key     *datastore.Key
		task    *PendingCloudTask
		taskObj *taskspb.Task
		sendErr error
	}
	var locked []lockedItem
	for _, key := range keys {
		task, acquired, err := acquireDispatchLock(ctx, key, handledBySweeper)
		if err != nil {
			logErrorf(ctx, "Failed to acquire lock for pending task %v: %v", key, err)
			continue
		}
		if !acquired {
			continue
		}

		taskObj, err := decodeTaskPayload(task.CloudTaskPayload)
		if err != nil {
			logErrorf(ctx, "Failed to dispatch task %s to queue %s: %v", task.CloudTaskName, task.QueueName, err)
			if recErr := recordDispatchFailure(ctx, key, err); recErr != nil {
				logErrorf(ctx, "Failed to record error state for task %s: %v", task.CloudTaskName, recErr)
			}
			continue
		}
		locked = append(locked, lockedItem{key: key, task: task, taskObj: taskObj})
	}

	if len(locked) == 1 {
		_, locked[0].sendErr = sendTask(ctx, locked[0].task.QueueName, locked[0].task.CloudTaskName, locked[0].taskObj)
	} else if len(locked) > 1 {
		var wg sync.WaitGroup
		for i := range locked {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, locked[idx].sendErr = sendTask(ctx, locked[idx].task.QueueName, locked[idx].task.CloudTaskName, locked[idx].taskObj)
			}(i)
		}
		wg.Wait()
	}

	count := 0
	for i := range locked {
		item := &locked[i]
		if item.sendErr != nil && item.sendErr != ErrTaskAlreadyAdded {
			logErrorf(ctx, "Failed to dispatch task %s to queue %s: %v", item.task.CloudTaskName, item.task.QueueName, item.sendErr)
			if recErr := recordDispatchFailure(ctx, item.key, item.sendErr); recErr != nil {
				logErrorf(ctx, "Failed to record error state for task %s: %v", item.task.CloudTaskName, recErr)
			}
			continue
		}

		// The task was created, or an earlier attempt already created it.
		if err := datastore.Delete(ctx, item.key); err != nil {
			logErrorf(ctx, "Failed to delete pending task %s from Datastore: %v", item.task.CloudTaskName, err)
		}
		count++
	}
	return count
}

func sweep(ctx context.Context) error {
	now := time.Now()
	var toDispatch, expiredFailed []*datastore.Key
	// Each status is read separately with a limit, so that memory use is
	// bounded and failed entities kept for inspection cannot crowd out pending
	// ones.
	for _, status := range []string{statusProcessing, statusPending, statusFailed} {
		var tasks []PendingCloudTask
		keys, err := datastore.NewQuery(pendingTaskKind).
			Filter("status =", status).
			Limit(sweepBatchSize).
			GetAll(ctx, &tasks)
		if err = ignoreFieldMismatch(err); err != nil {
			return fmt.Errorf("failed to query %s: %v", pendingTaskKind, err)
		}
		for i, key := range keys {
			task := &tasks[i]
			switch status {
			case statusFailed:
				failedAt := task.FailedAt
				if failedAt.IsZero() {
					failedAt = task.Created
				}
				if !failedAt.IsZero() && now.Sub(failedAt) > failedTaskRetention {
					expiredFailed = append(expiredFailed, key)
				}
				continue // exceeded max retries
			case statusPending:
				if !task.Created.IsZero() && now.Sub(task.Created) < fastPathGracePeriod {
					continue // give the post-commit dispatch time to run
				}
			}
			if isDispatchable(task, now) {
				toDispatch = append(toDispatch, key)
			}
		}
	}

	if len(expiredFailed) > 0 {
		if err := datastore.DeleteMulti(ctx, expiredFailed); err != nil {
			logErrorf(ctx, "Sweeper failed to delete expired failed tasks: %v", err)
		} else {
			log.Printf("Cloud Tasks sweeper deleted %d expired failed tasks.", len(expiredFailed))
		}
	}

	count := dispatchKeys(ctx, toDispatch, true)
	log.Printf("Cloud Tasks sweeper processed %d tasks.", count)
	return nil
}

func handleSweep(w http.ResponseWriter, r *http.Request) {
	isCron := strings.EqualFold(r.Header.Get("X-AppEngine-Cron"), "true") || strings.EqualFold(r.Header.Get("X-Appengine-Cron"), "true")
	if !isCron && !appengine.IsDevAppServer() {
		http.Error(w, "Access denied: endpoint only accessible via App Engine Cron.", http.StatusForbidden)
		return
	}
	ctx := appengine.NewContext(r)
	if err := sweep(ctx); err != nil {
		logErrorf(ctx, "Sweeper failed: %v", err)
		http.Error(w, fmt.Sprintf("Sweeper failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Sweeper completed successfully.\n"))
}
