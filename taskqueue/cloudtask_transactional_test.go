package taskqueue

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	oldproto "github.com/golang/protobuf/proto"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/internal"
	dspb "google.golang.org/appengine/internal/datastore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

// fakeDatastore is an in-memory datastore_v3 backend for the API calls made
// by transactional task staging and dispatch. Writes in a transaction are
// applied at commit.
type fakeDatastore struct {
	mu          sync.Mutex
	entities    map[string]*dspb.EntityProto
	staged      map[uint64][]func()
	nextID      int64
	nextHandle  uint64
	failCommits int // number of upcoming commits to fail as concurrent
}

func newFakeDatastore() *fakeDatastore {
	return &fakeDatastore{
		entities: make(map[string]*dspb.EntityProto),
		staged:   make(map[uint64][]func()),
	}
}

func refKey(r *dspb.Reference) string {
	var parts []string
	for _, e := range r.GetPath().GetElement() {
		parts = append(parts, fmt.Sprintf("%s:%d:%s", e.GetType(), e.GetId(), e.GetName()))
	}
	return strings.Join(parts, "/")
}

func (fd *fakeDatastore) context() context.Context {
	return internal.WithCallOverride(internal.ContextForTesting(&http.Request{}), fd.call)
}

func (fd *fakeDatastore) call(ctx context.Context, service, method string, in, out oldproto.Message) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	// Call overrides run before the transaction is applied to the request, so
	// the transaction is read from the context.
	txn := internal.TransactionFromContext(ctx)
	switch service + "." + method {
	case "__go__.GetNamespace":
		return nil
	case "datastore_v3.BeginTransaction":
		fd.nextHandle++
		h := fd.nextHandle
		out.(*dspb.Transaction).Handle = &h
		return nil
	case "datastore_v3.Commit":
		h := in.(*dspb.Transaction).GetHandle()
		ops := fd.staged[h]
		delete(fd.staged, h)
		if fd.failCommits > 0 {
			fd.failCommits--
			return &internal.APIError{Service: "datastore_v3", Code: int32(dspb.Error_CONCURRENT_TRANSACTION)}
		}
		for _, op := range ops {
			op()
		}
		return nil
	case "datastore_v3.Rollback":
		delete(fd.staged, in.(*dspb.Transaction).GetHandle())
		return nil
	case "datastore_v3.Put":
		req, res := in.(*dspb.PutRequest), out.(*dspb.PutResponse)
		for _, e := range req.Entity {
			e = oldproto.Clone(e).(*dspb.EntityProto)
			elems := e.GetKey().GetPath().GetElement()
			if last := elems[len(elems)-1]; last.GetId() == 0 && last.GetName() == "" {
				fd.nextID++
				id := fd.nextID
				last.Id = &id
			}
			res.Key = append(res.Key, oldproto.Clone(e.GetKey()).(*dspb.Reference))
			entity := e
			apply := func() { fd.entities[refKey(entity.GetKey())] = entity }
			if txn != nil {
				fd.staged[txn.GetHandle()] = append(fd.staged[txn.GetHandle()], apply)
			} else {
				apply()
			}
		}
		return nil
	case "datastore_v3.Get":
		req, res := in.(*dspb.GetRequest), out.(*dspb.GetResponse)
		for _, k := range req.Key {
			var e *dspb.EntityProto
			if stored, ok := fd.entities[refKey(k)]; ok {
				e = oldproto.Clone(stored).(*dspb.EntityProto)
			}
			res.Entity = append(res.Entity, &dspb.GetResponse_Entity{Entity: e, Key: k})
		}
		return nil
	case "datastore_v3.Delete":
		req := in.(*dspb.DeleteRequest)
		for _, k := range req.Key {
			key := refKey(k)
			apply := func() { delete(fd.entities, key) }
			if txn != nil {
				fd.staged[txn.GetHandle()] = append(fd.staged[txn.GetHandle()], apply)
			} else {
				apply()
			}
		}
		return nil
	case "datastore_v3.RunQuery":
		req, res := in.(*dspb.Query), out.(*dspb.QueryResult)
		// Results are in key (ID) order, like Datastore.
		var all []*dspb.EntityProto
		for _, e := range fd.entities {
			all = append(all, e)
		}
		sort.Slice(all, func(i, j int) bool { return entityID(all[i]) < entityID(all[j]) })
		for _, e := range all {
			if !fd.matches(e, req) {
				continue
			}
			if req.Limit != nil && int32(len(res.Result)) >= req.GetLimit() {
				break
			}
			res.Result = append(res.Result, oldproto.Clone(e).(*dspb.EntityProto))
		}
		res.MoreResults = oldproto.Bool(false)
		return nil
	}
	return fmt.Errorf("unexpected API call: %s.%s", service, method)
}

func entityID(e *dspb.EntityProto) int64 {
	elems := e.GetKey().GetPath().GetElement()
	return elems[len(elems)-1].GetId()
}

// matches supports a kind and equality filters on string properties, which
// is all the sweeper uses.
func (fd *fakeDatastore) matches(e *dspb.EntityProto, q *dspb.Query) bool {
	elems := e.GetKey().GetPath().GetElement()
	if elems[len(elems)-1].GetType() != q.GetKind() {
		return false
	}
	for _, f := range q.Filter {
		want := f.Property[0]
		found := false
		for _, p := range e.Property {
			if p.GetName() == want.GetName() && p.GetValue().GetStringValue() == want.GetValue().GetStringValue() {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (fd *fakeDatastore) pendingTasks(t *testing.T) map[string]PendingCloudTask {
	t.Helper()
	ctx := fd.context()
	var tasks []PendingCloudTask
	keys, err := datastore.NewQuery(pendingTaskKind).GetAll(ctx, &tasks)
	if err = ignoreFieldMismatch(err); err != nil {
		t.Fatalf("query pending tasks: %v", err)
	}
	m := make(map[string]PendingCloudTask)
	for i, k := range keys {
		m[k.Encode()] = tasks[i]
	}
	return m
}

func (fd *fakeDatastore) putPending(t *testing.T, task PendingCloudTask) *datastore.Key {
	t.Helper()
	payload, err := encodeTaskPayload(&taskspb.Task{})
	if err != nil {
		t.Fatal(err)
	}
	if task.CloudTaskName == "" {
		task.CloudTaskName = "tx-test"
	}
	if task.QueueName == "" {
		task.QueueName = "default"
	}
	if task.Status == "" {
		task.Status = statusPending
	}
	if task.Created.IsZero() {
		task.Created = time.Now().Add(-5 * time.Minute)
	}
	if task.CloudTaskPayload == "" {
		task.CloudTaskPayload = payload
	}
	ctx := fd.context()
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, pendingTaskKind, nil), &task)
	if err != nil {
		t.Fatalf("put pending task: %v", err)
	}
	return key
}

type order struct{ N int }

// setupTransactionalTest returns a fake datastore and a counter of CreateTask
// calls. createErr, if set, decides the error for each CreateTask call.
func setupTransactionalTest(t *testing.T, createErr func(*taskspb.CreateTaskRequest) error) (*fakeDatastore, *[]*taskspb.CreateTaskRequest) {
	t.Helper()
	var mu sync.Mutex
	var created []*taskspb.CreateTaskRequest
	v2Srv := &fakeCloudTasksV2Server{
		createTaskFunc: func(_ context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
			mu.Lock()
			created = append(created, req)
			mu.Unlock()
			if createErr != nil {
				if err := createErr(req); err != nil {
					return nil, err
				}
			}
			name := req.GetTask().GetName()
			if name == "" {
				name = req.GetParent() + "/tasks/auto"
			}
			return &taskspb.Task{Name: name}, nil
		},
	}
	_ = setupCloudTasksTestEnv(t, v2Srv, nil)
	pendingTasksMu.Lock()
	pendingTasks = make(map[uint64][]string)
	pendingTasksMu.Unlock()
	return newFakeDatastore(), &created
}

func pendingHandles() int {
	pendingTasksMu.Lock()
	defer pendingTasksMu.Unlock()
	return len(pendingTasks)
}

func TestTransactionalTask_DispatchedBeforeRunInTransactionReturns(t *testing.T) {
	fd, created := setupTransactionalTest(t, nil)
	ctx := fd.context()

	// Staged tasks are their own entity group, so the transaction is XG.
	err := datastore.RunInTransaction(ctx, func(tc context.Context) error {
		if _, err := datastore.Put(tc, datastore.NewIncompleteKey(tc, "Order", nil), &order{N: 1}); err != nil {
			return err
		}
		task, err := Add(tc, &Task{Path: "/worker"}, "default")
		if err != nil {
			return err
		}
		if !strings.HasPrefix(task.Name, "tx-") {
			t.Errorf("staged task name = %q, want a tx- name", task.Name)
		}
		return nil
	}, &datastore.TransactionOptions{XG: true})
	if err != nil {
		t.Fatalf("RunInTransaction: %v", err)
	}

	if len(*created) != 1 {
		t.Fatalf("CreateTask calls = %d, want 1 before RunInTransaction returns", len(*created))
	}
	if name := (*created)[0].GetTask().GetName(); !strings.Contains(name, "/tasks/tx-") {
		t.Errorf("CreateTask task name = %q, want a tx- name for deduplication", name)
	}
	if got := fd.pendingTasks(t); len(got) != 0 {
		t.Errorf("pending entities after dispatch = %v, want none", got)
	}
	if n := pendingHandles(); n != 0 {
		t.Errorf("pendingTasks handles = %d, want 0", n)
	}
}

func TestTransactionalTask_FailedCommitReleasesStagedTasks(t *testing.T) {
	fd, created := setupTransactionalTest(t, nil)
	ctx := fd.context()

	// A concurrent-transaction commit failure must clear what was staged.
	fd.failCommits = 1
	_, err := internal.RunTransactionOnce(ctx, func(tc context.Context) error {
		_, err := Add(tc, &Task{Path: "/worker"}, "default")
		return err
	}, false, false, nil)
	if err != internal.ErrConcurrentTransaction {
		t.Fatalf("RunTransactionOnce error = %v, want ErrConcurrentTransaction", err)
	}
	if n := pendingHandles(); n != 0 {
		t.Errorf("pendingTasks handles after concurrent commit failure = %d, want 0", n)
	}

	// RunInTransaction retries; only the committed attempt dispatches.
	fd.failCommits = 1
	err = datastore.RunInTransaction(ctx, func(tc context.Context) error {
		_, err := Add(tc, &Task{Path: "/worker"}, "default")
		return err
	}, nil)
	if err != nil {
		t.Fatalf("RunInTransaction: %v", err)
	}
	if len(*created) != 1 {
		t.Errorf("CreateTask calls = %d, want 1", len(*created))
	}
	if n := pendingHandles(); n != 0 {
		t.Errorf("pendingTasks handles = %d, want 0", n)
	}
	if got := fd.pendingTasks(t); len(got) != 0 {
		t.Errorf("pending entities = %v, want none", got)
	}
}

func TestDispatchKeys_LockingAndFailures(t *testing.T) {
	var failWith error
	var onCreate func()
	fd, created := setupTransactionalTest(t, func(*taskspb.CreateTaskRequest) error {
		if onCreate != nil {
			onCreate()
		}
		return failWith
	})
	ctx := fd.context()

	// A task locked by another dispatcher is skipped.
	locked := fd.putPending(t, PendingCloudTask{Status: statusProcessing, LockExpires: time.Now().Add(time.Minute)})
	dispatchKeys(ctx, []*datastore.Key{locked}, false)
	if len(*created) != 0 {
		t.Errorf("CreateTask calls for a locked task = %d, want 0", len(*created))
	}

	// Losing the lock transaction to a concurrent dispatcher skips the task.
	pending := fd.putPending(t, PendingCloudTask{})
	fd.failCommits = 3 // RunInTransaction makes 3 attempts.
	dispatchKeys(ctx, []*datastore.Key{pending}, false)
	if len(*created) != 0 {
		t.Errorf("CreateTask calls after losing the lock = %d, want 0", len(*created))
	}

	// Failures are recorded until the task is marked FAILED.
	failWith = status.Error(codes.Unavailable, "down")
	for i := 0; i < maxSweeperRetries; i++ {
		dispatchKeys(ctx, []*datastore.Key{pending}, true)
	}
	got := fd.pendingTasks(t)[pending.Encode()]
	if got.Status != statusFailed || got.RetryCount != maxSweeperRetries || got.FailedAt.IsZero() {
		t.Errorf("after %d failures: status=%q retries=%d failedAt=%v, want FAILED with failedAt", maxSweeperRetries, got.Status, got.RetryCount, got.FailedAt)
	}
	dispatchKeys(ctx, []*datastore.Key{pending}, true)
	if len(*created) != maxSweeperRetries {
		t.Errorf("CreateTask calls = %d, want %d (FAILED tasks are not dispatched)", len(*created), maxSweeperRetries)
	}

	// A failure does not recreate an entity another dispatcher deleted.
	deleted := fd.putPending(t, PendingCloudTask{})
	onCreate = func() {
		if err := datastore.Delete(ctx, deleted); err != nil {
			t.Errorf("delete: %v", err)
		}
	}
	dispatchKeys(ctx, []*datastore.Key{deleted}, false)
	if _, ok := fd.pendingTasks(t)[deleted.Encode()]; ok {
		t.Errorf("failed dispatch recreated an entity that was deleted meanwhile")
	}
	onCreate = nil

	// ALREADY_EXISTS means an earlier attempt created the task.
	exists := fd.putPending(t, PendingCloudTask{})
	failWith = status.Error(codes.AlreadyExists, "exists")
	dispatchKeys(ctx, []*datastore.Key{exists}, false)
	if _, ok := fd.pendingTasks(t)[exists.Encode()]; ok {
		t.Errorf("entity for an already created task was not deleted")
	}
}

func TestSweep_BoundedBatchOfEachStatusAndFailedRetention(t *testing.T) {
	fd, created := setupTransactionalTest(t, nil)
	ctx := fd.context()
	prev := sweepBatchSize
	sweepBatchSize = 2
	t.Cleanup(func() { sweepBatchSize = prev })

	now := time.Now()
	// Failed entities kept for inspection must not crowd out pending ones.
	for i := 0; i < 3; i++ {
		fd.putPending(t, PendingCloudTask{Status: statusFailed, RetryCount: maxSweeperRetries, FailedAt: now})
	}
	for i := 0; i < 3; i++ {
		fd.putPending(t, PendingCloudTask{})
	}
	fd.putPending(t, PendingCloudTask{Created: now}) // within the grace period

	if err := sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(*created) != 2 {
		t.Errorf("CreateTask calls after first sweep = %d, want 2", len(*created))
	}
	if err := sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(*created) != 3 {
		t.Errorf("CreateTask calls after second sweep = %d, want 3", len(*created))
	}

	statuses := map[string]int{}
	for _, task := range fd.pendingTasks(t) {
		statuses[task.Status]++
	}
	if statuses[statusFailed] != 3 || statuses[statusPending] != 1 || len(statuses) != 2 {
		t.Errorf("remaining statuses = %v, want 3 FAILED and 1 PENDING in its grace period", statuses)
	}

	// Failed entities are deleted after the retention period.
	sweepBatchSize = prev
	old := fd.putPending(t, PendingCloudTask{Status: statusFailed, RetryCount: maxSweeperRetries, FailedAt: now.Add(-failedTaskRetention - time.Hour)})
	if err := sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if _, ok := fd.pendingTasks(t)[old.Encode()]; ok {
		t.Errorf("failed entity past retention was not deleted")
	}
	if got := len(fd.pendingTasks(t)); got != 4 {
		t.Errorf("remaining entities = %d, want 4", got)
	}
}

func TestEmptyAddMultiAndDeleteMultiInCloudTasks(t *testing.T) {
	ctx := setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)
	tasks, err := AddMulti(ctx, nil, "default")
	if err != nil || len(tasks) != 0 {
		t.Errorf("AddMulti(nil) = %v, %v; want empty, nil", tasks, err)
	}
	if err := DeleteMulti(ctx, nil, "default"); err != nil {
		t.Errorf("DeleteMulti(nil) = %v, want nil", err)
	}
}

// Staged entities are shared with the Java and Python SDKs, so the sweeper
// dispatches tasks they staged, and tasks staged in the binary form used by
// earlier versions of this package.
func TestDispatchKeys_TasksStagedByOtherSDKs(t *testing.T) {
	fd, created := setupTransactionalTest(t, nil)
	ctx := fd.context()

	const queue = "projects/test-app/locations/us-central1/queues/default"
	binary, err := proto.Marshal(&taskspb.Task{
		Name: queue + "/tasks/tx-bin",
		MessageType: &taskspb.Task_AppEngineHttpRequest{AppEngineHttpRequest: &taskspb.AppEngineHttpRequest{
			HttpMethod:  taskspb.HttpMethod_POST,
			RelativeUri: "/bin",
			Body:        []byte("bin-body"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	payloads := map[string]string{
		// As written by the Java SDK. fieldAddedLater stands for a Task field
		// that is newer than the cloudtasks module this package uses.
		"/java": `{"task":{"name":"` + queue + `/tasks/tx-java","appEngineHttpRequest":{"appEngineRouting":{"service":"worker","version":"v1"},"httpMethod":"POST","relativeUri":"/java","body":"amF2YS1ib2R5","headers":{"X-Custom":"j"}},"retryConfig":{"maxAttempts":3,"minBackoff":"0.1s"},"scheduleTime":"2026-10-07T20:00:00Z","fieldAddedLater":1}}`,
		// As written by the Python SDK.
		"/py":  `{"task": {"name": "` + queue + `/tasks/tx-py", "appEngineHttpRequest": {"httpMethod": "PUT", "appEngineRouting": {"service": "worker", "version": "v1"}, "relativeUri": "/py", "headers": {"X-Custom": "p"}, "body": "cHktYm9keQ=="}, "scheduleTime": "2026-10-07T20:00:00.500Z", "retryConfig": {"maxAttempts": 3, "minBackoff": "2.300s"}}}`,
		"/bin": string(binary),
	}
	var keys []*datastore.Key
	for _, p := range payloads {
		keys = append(keys, fd.putPending(t, PendingCloudTask{CloudTaskPayload: p, SdkLang: "OTHER"}))
	}
	// An entity with a property this package does not declare.
	props := datastore.PropertyList{
		{Name: "queue_name", Value: "default"},
		{Name: "cloud_task_name", Value: "tx-extra"},
		{Name: "cloud_task_payload", Value: payloads["/py"], NoIndex: true},
		{Name: "status", Value: statusPending},
		{Name: "created", Value: time.Now().Add(-5 * time.Minute)},
		{Name: "sdk_lang", Value: "OTHER"},
		{Name: "added_by_a_later_version", Value: "x"},
	}
	extra, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, pendingTaskKind, nil), &props)
	if err != nil {
		t.Fatal(err)
	}

	if err := sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(*created) != len(keys)+1 {
		t.Fatalf("CreateTask calls = %d, want %d", len(*created), len(keys)+1)
	}
	if got := len(fd.pendingTasks(t)); got != 0 {
		t.Errorf("remaining entities = %d, want 0 (extra entity %v)", got, extra)
	}
	byURI := map[string]*taskspb.Task{}
	for _, req := range *created {
		byURI[req.GetTask().GetAppEngineHttpRequest().GetRelativeUri()] = req.GetTask()
	}
	java := byURI["/java"]
	if java == nil || string(java.GetAppEngineHttpRequest().GetBody()) != "java-body" ||
		java.GetAppEngineHttpRequest().GetHeaders()["X-Custom"] != "j" ||
		java.GetAppEngineHttpRequest().GetAppEngineRouting().GetVersion() != "v1" ||
		java.GetRetryConfig().GetMaxAttempts() != 3 ||
		!java.GetScheduleTime().AsTime().Equal(time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC)) {
		t.Errorf("Java-staged task dispatched as %v", java)
	}
	py := byURI["/py"]
	if py == nil || string(py.GetAppEngineHttpRequest().GetBody()) != "py-body" ||
		py.GetAppEngineHttpRequest().GetHttpMethod() != taskspb.HttpMethod_PUT ||
		py.GetRetryConfig().GetMinBackoff().AsDuration() != 2300*time.Millisecond ||
		py.GetName() != queue+"/tasks/tx-py" {
		t.Errorf("Python-staged task dispatched as %v", py)
	}
	if bin := byURI["/bin"]; bin == nil || string(bin.GetAppEngineHttpRequest().GetBody()) != "bin-body" {
		t.Errorf("binary-staged task dispatched as %v", bin)
	}
}
