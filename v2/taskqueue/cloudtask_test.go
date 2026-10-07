package taskqueue

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	oldproto "github.com/golang/protobuf/proto"
	"google.golang.org/api/option"
	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/datastore"
	"google.golang.org/appengine/v2/internal"
	"google.golang.org/appengine/v2/internal/aetesting"
	dspb "google.golang.org/appengine/v2/internal/datastore"
	pb "google.golang.org/appengine/v2/internal/taskqueue"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	taskspbbeta "cloud.google.com/go/cloudtasks/apiv2beta3/cloudtaskspb"
)

type fakeCloudTasksV2Server struct {
	taskspb.UnimplementedCloudTasksServer
	createTaskFunc       func(context.Context, *taskspb.CreateTaskRequest) (*taskspb.Task, error)
	batchCreateTasksFunc func(context.Context, *taskspb.BatchCreateTasksRequest) (*longrunningpb.Operation, error)
	batchDeleteTasksFunc func(context.Context, *taskspb.BatchDeleteTasksRequest) (*longrunningpb.Operation, error)
}

func (f *fakeCloudTasksV2Server) CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
	if f.createTaskFunc != nil {
		return f.createTaskFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (f *fakeCloudTasksV2Server) BatchCreateTasks(ctx context.Context, req *taskspb.BatchCreateTasksRequest) (*longrunningpb.Operation, error) {
	if f.batchCreateTasksFunc != nil {
		return f.batchCreateTasksFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (f *fakeCloudTasksV2Server) BatchDeleteTasks(ctx context.Context, req *taskspb.BatchDeleteTasksRequest) (*longrunningpb.Operation, error) {
	if f.batchDeleteTasksFunc != nil {
		return f.batchDeleteTasksFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

type fakeCloudTasksV2Beta3Server struct {
	taskspbbeta.UnimplementedCloudTasksServer
	getQueueFunc func(context.Context, *taskspbbeta.GetQueueRequest) (*taskspbbeta.Queue, error)
}

func (f *fakeCloudTasksV2Beta3Server) GetQueue(ctx context.Context, req *taskspbbeta.GetQueueRequest) (*taskspbbeta.Queue, error) {
	if f.getQueueFunc != nil {
		return f.getQueueFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func setupCloudTasksTestEnv(t *testing.T, v2Srv *fakeCloudTasksV2Server, betaSrv *fakeCloudTasksV2Beta3Server) context.Context {
	t.Helper()

	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing Metadata-Flavor", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("projects/12345/regions/us-central1"))
	}))
	t.Cleanup(metaSrv.Close)

	prevMetaURL := metadataRegionURL
	metadataRegionURL = metaSrv.URL
	t.Cleanup(func() { metadataRegionURL = prevMetaURL })

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	if v2Srv != nil {
		taskspb.RegisterCloudTasksServer(grpcSrv, v2Srv)
	}
	if betaSrv != nil {
		taskspbbeta.RegisterCloudTasksServer(grpcSrv, betaSrv)
	}
	go func() {
		_ = grpcSrv.Serve(lis)
	}()
	t.Cleanup(grpcSrv.Stop)

	prevOpts := cloudTasksClientOpts
	cloudTasksClientOpts = []option.ClientOption{
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
	resetCloudTasksClients()
	t.Cleanup(func() {
		resetCloudTasksClients()
		cloudTasksClientOpts = prevOpts
	})

	t.Setenv("APPENGINE_USE_CLOUDTASK_PUSH_QUEUE", "true")
	t.Setenv("GAE_APPLICATION", "s~test-app")

	return aetesting.FakeSingleContext(t, "taskqueue", "Add", func(_ *pb.TaskQueueAddRequest, _ *pb.TaskQueueAddResponse) error {
		t.Fatalf("legacy taskqueue RPC should not be called when APPENGINE_USE_CLOUDTASK_PUSH_QUEUE=true")
		return nil
	})
}

func TestBuildCloudTaskProto_BasicAndRouting(t *testing.T) {
	ctx := setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)

	hdr := make(http.Header)
	hdr.Set("Host", "worker-dot-test-app.appspot.com")
	hdr.Set("X-Custom", "val1")
	hdr.Set("X-AppEngine-QueueName", "spoofed")
	hdr.Set("X-Google-Foo", "bar")

	eta := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	task := &Task{
		Name:    "task-123",
		Path:    "/worker/push",
		Method:  "POST",
		Payload: []byte("hello-world"),
		Header:  hdr,
		ETA:     eta,
		RetryOptions: &RetryOptions{
			RetryLimit:            5,
			AgeLimit:              120 * time.Second,
			MinBackoff:            1500 * time.Millisecond,
			MaxBackoff:            30 * time.Second,
			MaxDoublings:          3,
			ApplyZeroMaxDoublings: false,
		},
	}

	protoTask, shortName, err := buildCloudTaskProto(ctx, "my-queue", task)
	if err != nil {
		t.Fatalf("buildCloudTaskProto failed: %v", err)
	}
	if shortName != "task-123" {
		t.Errorf("shortName = %q, want %q", shortName, "task-123")
	}
	wantFull := "projects/test-app/locations/us-central1/queues/my-queue/tasks/task-123"
	if protoTask.GetName() != wantFull {
		t.Errorf("protoTask.Name = %q, want %q", protoTask.GetName(), wantFull)
	}

	aeReq := protoTask.GetAppEngineHttpRequest()
	if aeReq == nil {
		t.Fatalf("AppEngineHttpRequest is nil")
	}
	if aeReq.GetRelativeUri() != "/worker/push" {
		t.Errorf("RelativeUri = %q, want /worker/push", aeReq.GetRelativeUri())
	}
	if aeReq.GetHttpMethod() != taskspb.HttpMethod_POST {
		t.Errorf("HttpMethod = %v, want POST", aeReq.GetHttpMethod())
	}
	if string(aeReq.GetBody()) != "hello-world" {
		t.Errorf("Body = %q, want hello-world", string(aeReq.GetBody()))
	}
	if aeReq.GetAppEngineRouting().GetService() != "worker" {
		t.Errorf("AppEngineRouting.Service = %q, want worker", aeReq.GetAppEngineRouting().GetService())
	}
	if _, hasHost := aeReq.GetHeaders()["Host"]; hasHost {
		t.Errorf("Host header should be stripped after routing extraction")
	}
	// Cloud Tasks sets the App Engine headers itself; reserved headers are
	// not sent.
	wantHeaders := map[string]string{
		"X-Custom":     "val1",
		"Content-Type": "application/octet-stream",
	}
	if got := aeReq.GetHeaders(); len(got) != len(wantHeaders) {
		t.Errorf("Headers = %v, want %v", got, wantHeaders)
	} else {
		for k, v := range wantHeaders {
			if got[k] != v {
				t.Errorf("Headers[%q] = %q, want %q", k, got[k], v)
			}
		}
	}
	if !protoTask.GetScheduleTime().AsTime().Equal(eta) {
		t.Errorf("ScheduleTime = %v, want %v", protoTask.GetScheduleTime().AsTime(), eta)
	}

	rc := protoTask.GetRetryConfig()
	if rc == nil {
		t.Fatalf("RetryConfig is nil")
	}
	if rc.GetMaxAttempts() != 5 {
		t.Errorf("MaxAttempts = %d, want 5", rc.GetMaxAttempts())
	}
	if rc.GetMaxRetryDuration().AsDuration() != 120*time.Second {
		t.Errorf("MaxRetryDuration = %v, want 120s", rc.GetMaxRetryDuration().AsDuration())
	}
	if rc.GetMinBackoff().AsDuration() != 1500*time.Millisecond {
		t.Errorf("MinBackoff = %v, want 1.5s", rc.GetMinBackoff().AsDuration())
	}
	if rc.GetMaxBackoff().AsDuration() != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want 30s", rc.GetMaxBackoff().AsDuration())
	}
	if rc.GetMaxDoublings() != 3 {
		t.Errorf("MaxDoublings = %d, want 3", rc.GetMaxDoublings())
	}
}

func TestRoutingFromHost(t *testing.T) {
	ctx := setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)
	t.Setenv("GAE_SERVICE", "current")
	tests := []struct {
		host                       string
		service, version, instance string
	}{
		{"", "current", "", ""},
		{"worker", "worker", "", ""},
		{"api.example.com", "current", "", ""},
		{"api.example.com:8080", "current", "", ""},
		{"test-app-other.example.com", "current", "", ""},
		{"test-app.appspot.com", "default", "", ""},
		{"worker-dot-test-app.appspot.com", "worker", "", ""},
		{"v2.worker.test-app.appspot.com", "worker", "v2", ""},
		{"v2-dot-worker-dot-test-app.appspot.com:443", "worker", "v2", ""},
		{"1.v2.worker.test-app.appspot.com", "worker", "v2", "1"},
		{"1-dot-v2-dot-worker-dot-test-app.uc.r.appspot.com", "worker", "v2", "1"},
		{"worker-dot-other-app.appspot.com", "current", "", ""},
		{"other-app.appspot.com", "current", "", ""},
	}
	for _, tc := range tests {
		r := routingFromHost(ctx, tc.host)
		if r.GetService() != tc.service || r.GetVersion() != tc.version || r.GetInstance() != tc.instance {
			t.Errorf("routingFromHost(%q) = %v, want service=%q version=%q instance=%q", tc.host, r, tc.service, tc.version, tc.instance)
		}
	}

	// Domain-scoped project ID: example.com:my-app -> my-app.example.com.appspot.com
	t.Setenv("GAE_APPLICATION", "s~example.com:my-app")
	domainTests := []struct {
		host                       string
		service, version, instance string
	}{
		{"my-app.example.com.appspot.com", "default", "", ""},
		{"worker.my-app.example.com.appspot.com", "worker", "", ""},
		{"v2-dot-worker-dot-my-app.example.com.uc.r.appspot.com", "worker", "v2", ""},
	}
	for _, tc := range domainTests {
		r := routingFromHost(ctx, tc.host)
		if r.GetService() != tc.service || r.GetVersion() != tc.version || r.GetInstance() != tc.instance {
			t.Errorf("routingFromHost(%q) with domain project = %v, want service=%q version=%q instance=%q", tc.host, r, tc.service, tc.version, tc.instance)
		}
	}
}

func TestNormalizeRegionAndMetadataCaching(t *testing.T) {
	ctx := setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)
	t.Setenv("LOCATION_ID", "us-central")
	r, err := getRegion(ctx)
	if err != nil || r != "us-central1" {
		t.Errorf("getRegion with LOCATION_ID=us-central = %q, %v; want us-central1, nil", r, err)
	}
	t.Setenv("LOCATION_ID", "europe-west")
	r, err = getRegion(ctx)
	if err != nil || r != "europe-west1" {
		t.Errorf("getRegion with LOCATION_ID=europe-west = %q, %v; want europe-west1, nil", r, err)
	}
}

func TestBuildCloudTaskProto_ValidationErrors(t *testing.T) {
	ctx := setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)

	if _, _, err := buildCloudTaskProto(ctx, "default", &Task{Name: "invalid name with spaces"}); err == nil {
		t.Errorf("expected error for invalid task name, got nil")
	}

	largePayload := make([]byte, maxTaskPayloadBytes+1)
	if _, _, err := buildCloudTaskProto(ctx, "default", &Task{Payload: largePayload}); err == nil {
		t.Errorf("expected error for payload > 100KB, got nil")
	}
}

func TestAddInCloudTasks_SingleTaskAndDuplicate(t *testing.T) {
	calls := 0
	v2Srv := &fakeCloudTasksV2Server{
		createTaskFunc: func(_ context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
			calls++
			if calls == 1 {
				return &taskspb.Task{
					Name: req.GetParent() + "/tasks/generated-1",
				}, nil
			}
			return nil, status.Error(codes.AlreadyExists, "Task already exists")
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	added, err := Add(ctx, &Task{Path: "/worker", Payload: []byte("p1")}, "default")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if added.Name != "generated-1" {
		t.Errorf("added.Name = %q, want generated-1", added.Name)
	}
	if added.Method != "POST" {
		t.Errorf("added.Method = %q, want POST", added.Method)
	}

	_, err = Add(ctx, &Task{Name: "generated-1", Path: "/worker"}, "default")
	if err != ErrTaskAlreadyAdded {
		t.Errorf("duplicate Add error = %v, want %v", err, ErrTaskAlreadyAdded)
	}
}

func TestAddMultiInCloudTasks_BatchCreateAndPartialFailure(t *testing.T) {
	v2Srv := &fakeCloudTasksV2Server{
		batchCreateTasksFunc: func(_ context.Context, req *taskspb.BatchCreateTasksRequest) (*longrunningpb.Operation, error) {
			if len(req.GetRequests()) != 3 {
				return nil, status.Errorf(codes.InvalidArgument, "expected 3 requests, got %d", len(req.GetRequests()))
			}
			respAny, err := anypb.New(&taskspb.BatchCreateTasksResponse{
				Tasks: []*taskspb.Task{
					{Name: req.GetParent() + "/tasks/batch-ok-1"},
					{Name: req.GetParent() + "/tasks/batch-ok-3"},
				},
			})
			if err != nil {
				return nil, err
			}
			metaAny, err := anypb.New(&taskspb.BatchCreateTasksMetadata{
				FailedRequests: map[int32]*rpcstatus.Status{
					1: {Code: int32(codes.AlreadyExists), Message: "Task already exists"},
				},
			})
			if err != nil {
				return nil, err
			}
			return &longrunningpb.Operation{
				Name:     "operations/batch-create-op-1",
				Done:     true,
				Metadata: metaAny,
				Result:   &longrunningpb.Operation_Response{Response: respAny},
			}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	tasks := []*Task{
		{Path: "/worker", Payload: []byte("1")},
		{Name: "dup-task", Path: "/worker", Payload: []byte("2")},
		{Path: "/worker", Payload: []byte("3")},
	}
	res, err := AddMulti(ctx, tasks, "default")
	me, ok := err.(appengine.MultiError)
	if !ok {
		t.Fatalf("expected appengine.MultiError, got %T (%v)", err, err)
	}
	if len(me) != 3 {
		t.Fatalf("len(MultiError) = %d, want 3", len(me))
	}
	if me[0] != nil || me[1] != ErrTaskAlreadyAdded || me[2] != nil {
		t.Errorf("MultiError = %v, want [nil, ErrTaskAlreadyAdded, nil]", me)
	}
	if res[0].Name != "batch-ok-1" {
		t.Errorf("res[0].Name = %q, want batch-ok-1", res[0].Name)
	}
	if res[2].Name != "batch-ok-3" {
		t.Errorf("res[2].Name = %q, want batch-ok-3", res[2].Name)
	}
}

func TestDeleteMultiInCloudTasks_BatchDeleteAndPartialFailure(t *testing.T) {
	callCount := 0
	v2Srv := &fakeCloudTasksV2Server{
		batchDeleteTasksFunc: func(_ context.Context, req *taskspb.BatchDeleteTasksRequest) (*longrunningpb.Operation, error) {
			callCount++
			respAny, _ := anypb.New(&emptypb.Empty{})
			var failed map[int32]*rpcstatus.Status
			if callCount == 2 {
				failed = map[int32]*rpcstatus.Status{
					0: {Code: int32(codes.NotFound), Message: "Task not found"},
				}
			}
			metaAny, _ := anypb.New(&taskspb.BatchDeleteTasksMetadata{
				FailedRequests: failed,
			})
			return &longrunningpb.Operation{
				Name:     "operations/batch-delete-op",
				Done:     true,
				Metadata: metaAny,
				Result:   &longrunningpb.Operation_Response{Response: respAny},
			}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	// 1. All succeed
	err := DeleteMulti(ctx, []*Task{{Name: "t1"}, {Name: "t2"}}, "default")
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	// 2. First task fails with NOT_FOUND -> mapped to UNKNOWN_TASK APIError
	err = DeleteMulti(ctx, []*Task{{Name: "missing"}, {Name: "present"}}, "default")
	me, ok := err.(appengine.MultiError)
	if !ok {
		t.Fatalf("expected appengine.MultiError, got %T (%v)", err, err)
	}
	apiErr, ok := me[0].(*internal.APIError)
	if !ok || apiErr.Code != int32(pb.TaskQueueServiceError_UNKNOWN_TASK) {
		t.Errorf("me[0] = %#v, want UNKNOWN_TASK APIError", me[0])
	}
	if me[1] != nil {
		t.Errorf("me[1] = %v, want nil", me[1])
	}
}

func TestQueueStatsInCloudTasks_V2Beta3(t *testing.T) {
	eta := time.Date(2026, 10, 6, 15, 0, 0, 0, time.UTC)
	betaSrv := &fakeCloudTasksV2Beta3Server{
		getQueueFunc: func(_ context.Context, req *taskspbbeta.GetQueueRequest) (*taskspbbeta.Queue, error) {
			if len(req.GetReadMask().GetPaths()) != 1 || req.GetReadMask().GetPaths()[0] != "stats" {
				return nil, status.Errorf(codes.InvalidArgument, "unexpected ReadMask: %v", req.GetReadMask())
			}
			return &taskspbbeta.Queue{
				Name: req.GetName(),
				Stats: &taskspbbeta.QueueStats{
					TasksCount:                 42,
					OldestEstimatedArrivalTime: timestamppb.New(eta),
					ExecutedLastMinuteCount:    7,
					ConcurrentDispatchesCount:  3,
					EffectiveExecutionRate:     12.5,
				},
			}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, nil, betaSrv)

	stats, err := QueueStats(ctx, []string{"default"})
	if err != nil {
		t.Fatalf("QueueStats failed: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("len(stats) = %d, want 1", len(stats))
	}
	if stats[0].Tasks != 42 || stats[0].Executed1Minute != 7 || stats[0].InFlight != 3 || stats[0].EnforcedRate != 12.5 {
		t.Errorf("unexpected stats[0]: %+v", stats[0])
	}
	if !stats[0].OldestETA.Equal(eta) {
		t.Errorf("OldestETA = %v, want %v", stats[0].OldestETA, eta)
	}
}

func TestTransactionalTasksInCloudTasks_StagingAndMaxLimit(t *testing.T) {
	_ = setupCloudTasksTestEnv(t, &fakeCloudTasksV2Server{}, nil)

	// Prevent background PostCommitHook goroutine from racing with test assertions
	prevHook := internal.PostCommitHook
	internal.PostCommitHook = nil
	t.Cleanup(func() { internal.PostCommitHook = prevHook })

	putCount := 0
	var lastSavedPayload []byte
	handle := uint64(999)
	t.Cleanup(func() { cleanupPendingTasks(handle) })

	c := internal.WithCallOverride(internal.ContextForTesting(&http.Request{}), func(_ context.Context, service, method string, in, out oldproto.Message) error {
		switch service + "." + method {
		case "__go__.GetNamespace":
			return nil
		case "datastore_v3.BeginTransaction":
			res := out.(*dspb.Transaction)
			res.Handle = &handle
			return nil
		case "datastore_v3.Commit":
			return nil
		case "datastore_v3.Rollback":
			return nil
		case "datastore_v3.Put":
			req := in.(*dspb.PutRequest)
			res := out.(*dspb.PutResponse)
			putCount++
			if len(req.Entity) > 0 {
				for _, prop := range append(req.Entity[0].Property, req.Entity[0].RawProperty...) {
					if prop.GetName() == "cloud_task_payload" {
						lastSavedPayload = []byte(prop.GetValue().GetStringValue())
					}
				}
			}
			id := int64(putCount)
			kind := "_AE_PendingCloudTask"
			app := "s~test-app"
			res.Key = []*dspb.Reference{
				{
					App: &app,
					Path: &dspb.Path{
						Element: []*dspb.Path_Element{
							{Type: &kind, Id: &id},
						},
					},
				},
			}
			return nil
		default:
			t.Fatalf("unexpected API call: %s.%s", service, method)
			return nil
		}
	})

	_, err := internal.RunTransactionOnce(c, func(txCtx context.Context) error {
		for i := 0; i < maxTransactionalTasks; i++ {
			_, err := Add(txCtx, &Task{
				Path:         "/worker/tx",
				Payload:      []byte("tx-body"),
				RetryOptions: &RetryOptions{RetryLimit: 4},
			}, "default")
			if err != nil {
				t.Fatalf("transactional Add #%d failed: %v", i+1, err)
			}
		}
		if putCount != maxTransactionalTasks {
			t.Errorf("putCount = %d, want %d", putCount, maxTransactionalTasks)
		}

		// 6th task in same transaction must fail with ErrTooManyTasksInTransaction
		_, err := Add(txCtx, &Task{Path: "/worker/tx"}, "default")
		if err != ErrTooManyTasksInTransaction {
			t.Errorf("6th task in transaction error = %v, want %v", err, ErrTooManyTasksInTransaction)
		}
		return nil
	}, false, false, nil)
	if err != nil {
		t.Fatalf("RunTransactionOnce failed: %v", err)
	}

	// The payload is {"task": T} with T the proto3 JSON form of the task,
	// like the Java and Python SDKs.
	// The Java SDK reads the JSON field names, not the proto field names.
	for _, want := range []string{`{"task":{`, `"appEngineHttpRequest":`, `"relativeUri":`, `"retryConfig":`} {
		if !strings.Contains(string(lastSavedPayload), want) {
			t.Errorf("staged payload = %s, want it to contain %s", lastSavedPayload, want)
		}
	}
	unmarshaled, err := decodeTaskPayload(string(lastSavedPayload))
	if err != nil {
		t.Fatalf("failed to decode staged task: %v", err)
	}
	if unmarshaled.GetRetryConfig().GetMaxAttempts() != 4 {
		t.Errorf("staged task MaxAttempts = %d, want 4", unmarshaled.GetRetryConfig().GetMaxAttempts())
	}
	if !strings.HasSuffix(unmarshaled.GetAppEngineHttpRequest().GetRelativeUri(), "/worker/tx") {
		t.Errorf("staged task RelativeUri = %q, want /worker/tx", unmarshaled.GetAppEngineHttpRequest().GetRelativeUri())
	}
}

// resetCloudTasksClients drops the cached clients so the next call creates
// clients with the current cloudTasksClientOpts.
func resetCloudTasksClients() {
	cloudTasksClientMu.Lock()
	defer cloudTasksClientMu.Unlock()
	if cloudTasksClient != nil {
		_ = cloudTasksClient.Close()
		cloudTasksClient = nil
	}
	if cloudTasksBetaClient != nil {
		_ = cloudTasksBetaClient.Close()
		cloudTasksBetaClient = nil
	}
	cachedRegion = ""
	cachedRegionURL = ""
}

func TestCloudTasksClient_ReusedAcrossCalls(t *testing.T) {
	v2Srv := &fakeCloudTasksV2Server{
		createTaskFunc: func(_ context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
			return &taskspb.Task{Name: req.GetParent() + "/tasks/t"}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	if _, err := Add(ctx, &Task{Path: "/worker"}, "default"); err != nil {
		t.Fatalf("first Add failed: %v", err)
	}
	first := cloudTasksClient
	if first == nil {
		t.Fatalf("client was not cached after first Add")
	}
	if _, err := Add(ctx, &Task{Path: "/worker"}, "default"); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}
	if cloudTasksClient != first {
		t.Errorf("client was recreated between calls")
	}
}

func TestAddMultiInCloudTasks_BuildErrorKeepsRequestIndexMapping(t *testing.T) {
	v2Srv := &fakeCloudTasksV2Server{
		batchCreateTasksFunc: func(_ context.Context, req *taskspb.BatchCreateTasksRequest) (*longrunningpb.Operation, error) {
			if len(req.GetRequests()) != 2 {
				return nil, status.Errorf(codes.InvalidArgument, "expected 2 requests, got %d", len(req.GetRequests()))
			}
			respAny, _ := anypb.New(&taskspb.BatchCreateTasksResponse{
				Tasks: []*taskspb.Task{{Name: req.GetParent() + "/tasks/created-0"}},
			})
			// Request index 1 is the third task, because the second task fails to build.
			metaAny, _ := anypb.New(&taskspb.BatchCreateTasksMetadata{
				FailedRequests: map[int32]*rpcstatus.Status{
					1: {Code: int32(codes.AlreadyExists), Message: "Task already exists"},
				},
			})
			return &longrunningpb.Operation{
				Name:     "operations/batch-create-op-2",
				Done:     true,
				Metadata: metaAny,
				Result:   &longrunningpb.Operation_Response{Response: respAny},
			}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	tasks := []*Task{
		{Path: "/worker"},
		{Name: "invalid name", Path: "/worker"},
		{Name: "dup", Path: "/worker"},
	}
	res, err := AddMulti(ctx, tasks, "default")
	me, ok := err.(appengine.MultiError)
	if !ok {
		t.Fatalf("expected appengine.MultiError, got %T (%v)", err, err)
	}
	if me[0] != nil {
		t.Errorf("me[0] = %v, want nil", me[0])
	}
	if me[1] == nil || me[1] == ErrTaskAlreadyAdded {
		t.Errorf("me[1] = %v, want build error", me[1])
	}
	if me[2] != ErrTaskAlreadyAdded {
		t.Errorf("me[2] = %v, want ErrTaskAlreadyAdded", me[2])
	}
	if res[0] == nil || res[0].Name != "created-0" {
		t.Errorf("res[0] = %+v, want Name created-0", res[0])
	}
}

func TestMapOperationErrorCode_NotFoundOnCreateIsUnknownQueue(t *testing.T) {
	err := mapOperationErrorCode(int(codes.NotFound), "Requested entity was not found.", false)
	if err == ErrTaskAlreadyAdded {
		t.Fatalf("NOT_FOUND on create mapped to ErrTaskAlreadyAdded")
	}
	apiErr, ok := err.(*internal.APIError)
	if !ok || apiErr.Code != int32(pb.TaskQueueServiceError_UNKNOWN_QUEUE) {
		t.Errorf("err = %#v, want UNKNOWN_QUEUE APIError", err)
	}
	if got := mapOperationErrorCode(int(codes.AlreadyExists), "Task already exists", false); got != ErrTaskAlreadyAdded {
		t.Errorf("ALREADY_EXISTS mapped to %v, want ErrTaskAlreadyAdded", got)
	}
}

func TestIgnoreFieldMismatch_MultiError(t *testing.T) {
	fm := &datastore.ErrFieldMismatch{FieldName: "extra"}
	if got := ignoreFieldMismatch(fm); got != nil {
		t.Errorf("ignoreFieldMismatch(ErrFieldMismatch) = %v, want nil", got)
	}
	if got := ignoreFieldMismatch(appengine.MultiError{fm, nil, fm}); got != nil {
		t.Errorf("ignoreFieldMismatch(MultiError of ErrFieldMismatch) = %v, want nil", got)
	}
	otherErr := datastore.ErrNoSuchEntity
	got := ignoreFieldMismatch(appengine.MultiError{fm, otherErr})
	me, ok := got.(appengine.MultiError)
	if !ok || me[0] != nil || me[1] != otherErr {
		t.Errorf("ignoreFieldMismatch(MultiError with real error) = %v, want [nil, ErrNoSuchEntity]", got)
	}
}

func TestAddMultiInCloudTasks_MultiChunkConcurrent(t *testing.T) {
	v2Srv := &fakeCloudTasksV2Server{
		batchCreateTasksFunc: func(_ context.Context, req *taskspb.BatchCreateTasksRequest) (*longrunningpb.Operation, error) {
			created := make([]*taskspb.Task, len(req.GetRequests()))
			for i, r := range req.GetRequests() {
				created[i] = &taskspb.Task{
					Name: req.GetParent() + "/tasks/" + strings.TrimPrefix(r.GetTask().GetAppEngineHttpRequest().GetRelativeUri(), "/"),
				}
			}
			respAny, err := anypb.New(&taskspb.BatchCreateTasksResponse{Tasks: created})
			if err != nil {
				return nil, err
			}
			metaAny, err := anypb.New(&taskspb.BatchCreateTasksMetadata{})
			if err != nil {
				return nil, err
			}
			return &longrunningpb.Operation{
				Name:     "operations/batch-create-multi",
				Done:     true,
				Metadata: metaAny,
				Result:   &longrunningpb.Operation_Response{Response: respAny},
			}, nil
		},
	}
	ctx := setupCloudTasksTestEnv(t, v2Srv, nil)

	const total = 250
	tasks := make([]*Task, total)
	for i := 0; i < total; i++ {
		tasks[i] = &Task{Path: "/t-" + string(rune('a'+(i%26)))}
	}
	res, err := AddMulti(ctx, tasks, "default")
	if err != nil {
		t.Fatalf("AddMulti(250 tasks) failed: %v", err)
	}
	for i := 0; i < total; i++ {
		want := "t-" + string(rune('a'+(i%26)))
		if res[i] == nil || res[i].Name != want {
			t.Fatalf("res[%d] = %+v, want Name %q", i, res[i], want)
		}
	}
}
