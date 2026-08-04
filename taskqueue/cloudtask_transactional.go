package taskqueue

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/internal"
	"google.golang.org/protobuf/proto"

	taskspb "cloud.google.com/go/cloudtasks/apiv2beta3/cloudtaskspb"
)

const (
	statusPending       = "PENDING"
	statusProcessing    = "PROCESSING"
	statusFailed        = "FAILED"
	statusDone          = "DONE"
	statusAlreadyExists = "ALREADY_EXISTS"

	lockDuration        = 60 * time.Second
	fastPathGracePeriod = 60 * time.Second
	maxSweeperRetries   = 5
	maxLastErrorLength  = 500
)

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
	SdkLang          string    `datastore:"sdk_lang"`
}

var (
	pendingTasksMu sync.Mutex
	pendingTasks   = make(map[uint64][]string) // transaction handle -> list of urlsafe keys
)

func init() {
	internal.PostCommitHook = func(ctx context.Context, handle uint64) {
		go dispatchPendingTasks(ctx, handle)
	}
	internal.RollbackHook = func(handle uint64) {
		cleanupPendingTasks(handle)
	}
	http.HandleFunc("/_ah/cloudtask/sweep", handleSweep)
}

func cleanupPendingTasks(handle uint64) {
	pendingTasksMu.Lock()
	delete(pendingTasks, handle)
	pendingTasksMu.Unlock()
}

type noCancelContext struct {
	context.Context
}

func (c *noCancelContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (c *noCancelContext) Done() <-chan struct{} {
	return nil
}

func (c *noCancelContext) Err() error {
	return nil
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

	noCancelCtx := &noCancelContext{Context: internal.TransactionlessContext(ctx)}

	for _, urlsafeKey := range urlsafeKeys {
		key, err := datastore.DecodeKey(urlsafeKey)
		if err != nil {
			logErrorf(ctx, "Failed to decode pending task key: %v", err)
			continue
		}

		var taskEntity PendingCloudTask
		err = datastore.Get(noCancelCtx, key, &taskEntity)
		if err != nil {
			logErrorf(ctx, "Failed to get pending task from Datastore: %v", err)
			continue
		}

		now := time.Now()
		taskEntity.Status = statusProcessing
		taskEntity.LockExpires = now.Add(lockDuration)
		taskEntity.HandledBySweeper = false
		if _, err := datastore.Put(noCancelCtx, key, &taskEntity); err != nil {
			logErrorf(ctx, "Failed to acquire lock in fast-path for task %s: %v", taskEntity.CloudTaskName, err)
			continue
		}

		var taskObj taskspb.Task
		if err := proto.Unmarshal([]byte(taskEntity.CloudTaskPayload), &taskObj); err != nil {
			logErrorf(ctx, "Failed to unmarshal pending task proto: %v", err)
			continue
		}
		_, err = sendTask(noCancelCtx, taskEntity.QueueName, taskEntity.CloudTaskName, &taskObj)
		if err != nil {
			if err == ErrTaskAlreadyAdded {
				datastore.Delete(noCancelCtx, key)
				continue
			}
			logErrorf(ctx, "Failed to dispatch task %s to queue %s: %v", taskEntity.CloudTaskName, taskEntity.QueueName, err)
			taskEntity.RetryCount++
			taskEntity.LastError = err.Error()
			if len(taskEntity.LastError) > maxLastErrorLength {
				taskEntity.LastError = taskEntity.LastError[:maxLastErrorLength]
			}
			taskEntity.Status = statusPending
			datastore.Put(noCancelCtx, key, &taskEntity)
			continue
		}

		err = datastore.Delete(noCancelCtx, key)
		if err != nil {
			logErrorf(ctx, "Failed to delete pending task %s from Datastore: %v", taskEntity.CloudTaskName, err)
		}
	}
}

func sweep(ctx context.Context) error {
	query := datastore.NewQuery("_AE_PendingCloudTask")
	var tasks []PendingCloudTask
	keys, err := query.GetAll(ctx, &tasks)
	if err != nil {
		return fmt.Errorf("failed to query _AE_PendingCloudTask: %v", err)
	}

	now := time.Now()
	count := 0
	for i, key := range keys {
		task := tasks[i]
		if task.Status == statusDone || task.Status == statusAlreadyExists {
			continue
		}
		if task.Status == statusProcessing {
			if !task.LockExpires.IsZero() && now.Before(task.LockExpires) {
				continue // Still actively processing and lock valid
			} else if task.LockExpires.IsZero() {
				continue // Assume lock valid if just started
			}
		} else if task.Status == statusPending || task.Status == "" {
			if !task.Created.IsZero() && now.Sub(task.Created) < fastPathGracePeriod {
				continue // Give fast-path grace period to dispatch post-commit
			}
		} else if task.Status == statusFailed && task.RetryCount >= maxSweeperRetries {
			continue // Exceeded max sweeper retries
		}

		// Acquire lock
		task.Status = statusProcessing
		task.LockExpires = now.Add(lockDuration)
		task.HandledBySweeper = true
		if _, err := datastore.Put(ctx, key, &task); err != nil {
			logErrorf(ctx, "Sweeper failed to acquire lock for task %s: %v", task.CloudTaskName, err)
			continue
		}

		var taskObj taskspb.Task
		if err := proto.Unmarshal([]byte(task.CloudTaskPayload), &taskObj); err != nil {
			logErrorf(ctx, "Sweeper failed to unmarshal pending task proto: %v", err)
			continue
		}
		_, err := sendTask(ctx, task.QueueName, task.CloudTaskName, &taskObj)
		if err != nil && err != ErrTaskAlreadyAdded {
			logErrorf(ctx, "Sweeper failed to dispatch task %s: %v", task.CloudTaskName, err)
			task.RetryCount++
			task.LastError = err.Error()
			if len(task.LastError) > maxLastErrorLength {
				task.LastError = task.LastError[:maxLastErrorLength]
			}
			if task.RetryCount >= maxSweeperRetries {
				task.Status = statusFailed
				task.LockExpires = time.Time{}
			} else {
				task.Status = statusPending
				task.LockExpires = time.Time{}
			}
			if _, putErr := datastore.Put(ctx, key, &task); putErr != nil {
				logErrorf(ctx, "Sweeper failed to record error state for task %s: %v", task.CloudTaskName, putErr)
			}
			continue
		}

		if err := datastore.Delete(ctx, key); err != nil {
			logErrorf(ctx, "Sweeper failed to delete entity %s: %v", task.CloudTaskName, err)
		}
		count++
	}

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
