package integration_test

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	blockDurabilityIntegrationEnv = "ZONDSCAN_BLOCK_DURABILITY_INTEGRATION"
	blockDurabilityHelperEnv      = "ZONDSCAN_BLOCK_DURABILITY_HELPER"
	orderedPartialIndexName       = "integration_ordered_partial_unique"
)

func TestBlockDurabilityCrashReplayAndOfflineMigration(t *testing.T) {
	if phase := os.Getenv(blockDurabilityHelperEnv); phase != "" {
		runBlockDurabilityHelper(t, phase)
		return
	}
	if os.Getenv(blockDurabilityIntegrationEnv) != "1" {
		t.Skipf("set %s=1 to run the isolated Mongo replica-set integration test",
			blockDurabilityIntegrationEnv)
	}

	moduleRoot := integrationModuleRoot(t)
	port := reserveLoopbackPort(t)
	containerName := fmt.Sprintf("zondscan-block-durability-it-%d", os.Getpid())
	dockerRun(t, moduleRoot,
		"run", "--rm", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%d:27017", port),
		"mongo:7",
		"--replSet", "rs0",
		"--bind_ip_all",
		"--setParameter", "enableTestCommands=1",
	)
	t.Cleanup(func() {
		_, _ = exec.Command("docker", "rm", "-f", containerName).CombinedOutput()
	})

	waitForMongoShell(t, moduleRoot, containerName)
	dockerRun(t, moduleRoot, "exec", containerName, "mongosh", "--quiet", "--eval",
		`rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]}).ok`)
	waitForMongoPrimary(t, moduleRoot, containerName)

	mongoURI := fmt.Sprintf(
		"mongodb://127.0.0.1:%d/?replicaSet=rs0&directConnection=true&retryWrites=false",
		port,
	)
	dropIntegrationDatabase(t, mongoURI)

	runIntegrationHelperProcess(t, moduleRoot, mongoURI, "ordered-partial")
	assertBlockStates(t, mongoURI, map[int64]string{0: db.BlockIngestionPending})

	dockerRun(t, moduleRoot, "restart", containerName)
	waitForMongoPrimary(t, moduleRoot, containerName)
	runIntegrationHelperProcess(t, moduleRoot, mongoURI, "restart-replay")
	assertBlockStates(t, mongoURI, map[int64]string{
		0: db.BlockIngestionComplete,
		1: db.BlockIngestionComplete,
		2: db.BlockIngestionComplete,
	})
	assertMigrationAttestation(t, mongoURI, "offline-reindex")

	dropIntegrationDatabase(t, mongoURI)
	seedLegacyBlocks(t, mongoURI, 3)
	rpcServer, requestedHeights := newMigrationRPCServer(t, 3)
	defer rpcServer.Close()

	dryRun := runMigrationCommand(t, moduleRoot, mongoURI, rpcServer.URL)
	if !strings.Contains(dryRun, "3 non-complete rows, 0 missing heights, first block=0x0") {
		t.Fatalf("migration dry-run output did not inventory markerless rows:\n%s", dryRun)
	}

	checkpoint := runMigrationCommand(t, moduleRoot, mongoURI, rpcServer.URL, "--execute", "--limit", "1")
	if !strings.Contains(checkpoint, "checkpoint complete; next block=0x1") {
		t.Fatalf("limited migration did not report its resume checkpoint:\n%s", checkpoint)
	}
	assertBlockStates(t, mongoURI, map[int64]string{
		0: db.BlockIngestionComplete,
		1: "",
		2: "",
	})

	completed := runMigrationCommand(t, moduleRoot, mongoURI, rpcServer.URL, "--execute")
	if !strings.Contains(completed, "block companion reindex complete: migration gate is clear") {
		t.Fatalf("resumed migration did not clear its durable gate:\n%s", completed)
	}
	assertBlockStates(t, mongoURI, map[int64]string{
		0: db.BlockIngestionComplete,
		1: db.BlockIngestionComplete,
		2: db.BlockIngestionComplete,
	})
	assertMigrationAttestation(t, mongoURI, "offline-reindex")
	assertLeaseReleased(t, mongoURI)

	gotHeights := requestedHeights()
	wantHeights := []string{"0x0", "0x1", "0x2"}
	if strings.Join(gotHeights, ",") != strings.Join(wantHeights, ",") {
		t.Fatalf("migration RPC order = %v, want %v", gotHeights, wantHeights)
	}
	t.Logf("validated ordered partial-write recovery, failCommand uncertainty, restart replay, and migration checkpoint resume in %s on 127.0.0.1:%d",
		containerName, port)
}

func runBlockDurabilityHelper(t *testing.T, phase string) {
	if err := configs.ConnectDB(); err != nil {
		t.Fatalf("connect helper to MongoDB: %v", err)
	}
	switch phase {
	case "ordered-partial":
		runOrderedPartialHelper(t)
	case "restart-replay":
		runRestartReplayHelper(t)
	default:
		t.Fatalf("unknown block durability helper phase %q", phase)
	}
}

func runOrderedPartialHelper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err := configs.BlocksCollections.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "integrationOrderedPartial", Value: 1}},
		Options: options.Index().SetName(orderedPartialIndexName).SetUnique(true),
	})
	if err != nil {
		t.Fatalf("create deterministic partial-write index: %v", err)
	}

	blocks := integrationBlocks(0, 3)
	result, err := db.InsertManyBlockDocuments(blocks)
	if !errors.Is(err, db.ErrBlockWriteUnresolved) {
		t.Fatalf("ordered partial write error = %v, want ErrBlockWriteUnresolved", err)
	}
	if result.Attempted != 3 || len(result.Confirmed) != 1 ||
		result.Confirmed[0].Block.Result.Number != "0x0" {
		t.Fatalf("ordered partial result = %#v, want only block 0x0 confirmed", result)
	}
	if _, err := configs.BlocksCollections.Indexes().DropOne(ctx, orderedPartialIndexName); err != nil {
		t.Fatalf("drop deterministic partial-write index: %v", err)
	}

	admin := configs.DB.Database("admin")
	failpoint := bson.D{
		{Key: "configureFailPoint", Value: "failCommand"},
		{Key: "mode", Value: bson.D{{Key: "times", Value: 1}}},
		{Key: "data", Value: bson.D{
			{Key: "failCommands", Value: bson.A{"insert"}},
			{Key: "closeConnection", Value: true},
		}},
	}
	if err := admin.RunCommand(ctx, failpoint).Err(); err != nil {
		t.Fatalf("enable failCommand connection drop: %v", err)
	}
	uncertain, uncertainErr := db.InsertManyBlockDocuments(integrationBlocks(3, 2))
	_ = admin.RunCommand(ctx, bson.D{
		{Key: "configureFailPoint", Value: "failCommand"},
		{Key: "mode", Value: "off"},
	}).Err()

	durableCount, countErr := configs.BlocksCollections.CountDocuments(ctx, bson.M{
		"blockNumberInt": bson.M{"$in": bson.A{int64(3), int64(4)}},
	})
	if countErr != nil {
		t.Fatalf("count failCommand batch outcome: %v", countErr)
	}
	if int64(len(uncertain.Confirmed)) != durableCount {
		t.Fatalf("failCommand confirmed %d blocks with %d durable rows",
			len(uncertain.Confirmed), durableCount)
	}
	if uncertainErr == nil && durableCount != 2 {
		t.Fatalf("failCommand write returned nil with %d of 2 rows durable", durableCount)
	}
	if uncertainErr != nil && !errors.Is(uncertainErr, db.ErrBlockWriteUnresolved) {
		t.Fatalf("failCommand write error = %v, want ErrBlockWriteUnresolved", uncertainErr)
	}
	if _, err := configs.BlocksCollections.DeleteMany(ctx, bson.M{
		"blockNumberInt": bson.M{"$in": bson.A{int64(3), int64(4)}},
	}); err != nil {
		t.Fatalf("remove failCommand probe rows: %v", err)
	}
	t.Logf("server ordered write retained 1/3 rows; failCommand retained %d/2 rows and the post-write read classified %d",
		durableCount, len(uncertain.Confirmed))
}

func runRestartReplayHelper(t *testing.T) {
	result, err := db.InsertManyBlockDocuments(integrationBlocks(0, 3))
	if err != nil {
		t.Fatalf("replay pending ordered prefix: %v", err)
	}
	if result.Attempted != 3 || len(result.Confirmed) != 3 {
		t.Fatalf("restart replay result = %#v, want all three blocks confirmed", result)
	}
	for index, confirmed := range result.Confirmed {
		wantExisting := index == 0
		if confirmed.AlreadyExisted != wantExisting {
			t.Fatalf("block %s AlreadyExisted = %v, want %v",
				confirmed.Block.Result.Number, confirmed.AlreadyExisted, wantExisting)
		}
		if err := db.MarkBlockCompanionsComplete(
			confirmed.Block.Result.Number,
			confirmed.Block.Result.Hash,
		); err != nil {
			t.Fatalf("complete replayed block %s: %v", confirmed.Block.Result.Number, err)
		}
	}
	if err := db.StoreLastKnownBlockNumber("0x2"); err != nil {
		t.Fatalf("store replayed durable cursor: %v", err)
	}
	missing, err := db.AuditCompletedCanonicalBlockChain()
	if err != nil {
		t.Fatalf("audit replayed canonical chain: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("replayed canonical chain has missing heights: %v", missing)
	}
	if err := db.EnsureUniqueBlockHeightIndex(); err != nil {
		t.Fatalf("enforce unique block height after replay audit: %v", err)
	}
	if err := db.MarkBlockIngestionMigrationComplete("0x2"); err != nil {
		t.Fatalf("attest replayed canonical chain: %v", err)
	}
	if err := db.ValidateBlockIngestionMigration(); err != nil {
		t.Fatalf("validate replayed ingestion boundary: %v", err)
	}
}

func integrationBlocks(start, count int) []models.ZondDatabaseBlock {
	blocks := make([]models.ZondDatabaseBlock, 0, count)
	for height := start; height < start+count; height++ {
		blocks = append(blocks, integrationBlock(height))
	}
	return blocks
}

func integrationBlock(height int) models.ZondDatabaseBlock {
	parentHash := "0x" + strings.Repeat("0", 64)
	if height > 0 {
		parentHash = integrationBlockHash(height - 1)
	}
	return models.ZondDatabaseBlock{
		Jsonrpc: "2.0",
		ID:      1,
		Result: models.Result{
			Hash:         integrationBlockHash(height),
			Number:       fmt.Sprintf("0x%x", height),
			ParentHash:   parentHash,
			Timestamp:    fmt.Sprintf("0x%x", height+1),
			Transactions: []models.Transaction{},
			Size:         "0x0",
		},
	}
}

func integrationBlockHash(height int) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("zondscan-block-durability-%d", height)))
	return "0x" + hex.EncodeToString(digest[:])
}

func integrationModuleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test source path")
	}
	return filepath.Dir(filepath.Dir(filename))
}

func reserveLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func dockerRun(t *testing.T, workdir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func waitForMongoShell(t *testing.T, workdir, containerName string) {
	t.Helper()
	waitForCondition(t, 30*time.Second, func() bool {
		cmd := exec.Command("docker", "exec", containerName, "mongosh", "--quiet", "--eval",
			`db.adminCommand({ping:1}).ok`)
		cmd.Dir = workdir
		output, err := cmd.CombinedOutput()
		return err == nil && strings.TrimSpace(string(output)) == "1"
	}, "Mongo shell readiness")
}

func waitForMongoPrimary(t *testing.T, workdir, containerName string) {
	t.Helper()
	waitForCondition(t, 45*time.Second, func() bool {
		cmd := exec.Command("docker", "exec", containerName, "mongosh", "--quiet", "--eval",
			`db.hello().isWritablePrimary`)
		cmd.Dir = workdir
		output, err := cmd.CombinedOutput()
		return err == nil && strings.TrimSpace(string(output)) == "true"
	}, "Mongo replica-set primary")
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, label string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func runIntegrationHelperProcess(t *testing.T, workdir, mongoURI, phase string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve integration test executable: %v", err)
	}
	cmd := exec.Command(executable,
		"-test.run=^TestBlockDurabilityCrashReplayAndOfflineMigration$",
		"-test.v",
	)
	cmd.Dir = workdir
	cmd.Env = replaceEnvironment(os.Environ(), map[string]string{
		"MONGOURI":                    mongoURI,
		blockDurabilityIntegrationEnv: "1",
		blockDurabilityHelperEnv:      phase,
	})
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("integration helper phase %s: %v\n%s", phase, err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "server ordered write retained") {
			t.Log(strings.TrimSpace(line))
		}
	}
	t.Logf("helper phase %s passed", phase)
}

func replaceEnvironment(base []string, replacements map[string]string) []string {
	environment := make([]string, 0, len(base)+len(replacements))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := replacements[key]; !replace {
			environment = append(environment, entry)
		}
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		environment = append(environment, key+"="+replacements[key])
	}
	return environment
}

func connectIntegrationMongo(t *testing.T, mongoURI string) *mongo.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("connect integration MongoDB: %v", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		t.Fatalf("ping integration MongoDB: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})
	return client
}

func dropIntegrationDatabase(t *testing.T, mongoURI string) {
	t.Helper()
	client := connectIntegrationMongo(t, mongoURI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Database("qrldata-z").Drop(ctx); err != nil {
		t.Fatalf("drop isolated integration database: %v", err)
	}
}

func assertBlockStates(t *testing.T, mongoURI string, expected map[int64]string) {
	t.Helper()
	client := connectIntegrationMongo(t, mongoURI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := client.Database("qrldata-z").Collection("blocks").Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "blockNumberInt", Value: 1}}),
	)
	if err != nil {
		t.Fatalf("read block states: %v", err)
	}
	defer cursor.Close(ctx)
	type blockState struct {
		BlockNumberInt      int64     `bson:"blockNumberInt"`
		IngestionState      string    `bson:"ingestionState"`
		TokenIngestionState string    `bson:"tokenIngestionState"`
		TokenCompletedAt    time.Time `bson:"tokenCompletedAt"`
	}
	var states []blockState
	if err := cursor.All(ctx, &states); err != nil {
		t.Fatalf("decode block states: %v", err)
	}
	if len(states) != len(expected) {
		t.Fatalf("block state row count = %d, want %d: %#v", len(states), len(expected), states)
	}
	for _, state := range states {
		want, ok := expected[state.BlockNumberInt]
		if !ok {
			t.Fatalf("unexpected block height %d in %#v", state.BlockNumberInt, states)
		}
		if state.IngestionState != want {
			t.Fatalf("block %d ingestionState = %q, want %q",
				state.BlockNumberInt, state.IngestionState, want)
		}
		if want == db.BlockIngestionComplete &&
			(state.TokenIngestionState != "complete" || state.TokenCompletedAt.IsZero()) {
			t.Fatalf("empty complete block %d token state = %q completedAt=%v",
				state.BlockNumberInt, state.TokenIngestionState, state.TokenCompletedAt)
		}
	}
}

func assertMigrationAttestation(t *testing.T, mongoURI, expectedOrigin string) {
	t.Helper()
	client := connectIntegrationMongo(t, mongoURI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var attestation struct {
		Schema  string `bson:"schema"`
		Status  string `bson:"status"`
		Through string `bson:"audited_through"`
		Origin  string `bson:"origin"`
	}
	err := client.Database("qrldata-z").Collection("sync_state").FindOne(
		ctx,
		bson.M{"_id": db.BlockIngestionMigrationID},
	).Decode(&attestation)
	if err != nil {
		t.Fatalf("read migration attestation: %v", err)
	}
	if attestation.Schema != db.BlockIngestionMigrationID ||
		attestation.Status != db.BlockIngestionComplete ||
		attestation.Through != "0x2" || attestation.Origin != expectedOrigin {
		t.Fatalf("migration attestation = %#v", attestation)
	}
}

func seedLegacyBlocks(t *testing.T, mongoURI string, count int) {
	t.Helper()
	client := connectIntegrationMongo(t, mongoURI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	documents := make([]interface{}, 0, count)
	for height := 0; height < count; height++ {
		block := integrationBlock(height)
		document := bson.M{
			"jsonrpc":        block.Jsonrpc,
			"id":             block.ID,
			"blockNumberInt": int64(height),
			"result": bson.M{
				"number":       block.Result.Number,
				"hash":         block.Result.Hash,
				"parenthash":   block.Result.ParentHash,
				"timestamp":    block.Result.Timestamp,
				"transactions": bson.A{},
				"size":         block.Result.Size,
			},
		}
		if height == 0 {
			document["ingestionState"] = db.BlockIngestionPending
		}
		documents = append(documents, document)
	}
	database := client.Database("qrldata-z")
	if _, err := database.Collection("blocks").InsertMany(ctx, documents); err != nil {
		t.Fatalf("seed markerless legacy blocks: %v", err)
	}
	if _, err := database.Collection("sync_state").InsertOne(ctx, bson.M{
		"_id":              db.LastSyncedBlockID,
		"block_number":     fmt.Sprintf("0x%x", count-1),
		"block_number_int": int64(count - 1),
	}); err != nil {
		t.Fatalf("seed legacy durable cursor: %v", err)
	}
}

func newMigrationRPCServer(t *testing.T, count int) (*httptest.Server, func() []string) {
	t.Helper()
	blocks := make(map[string]models.ZondDatabaseBlock, count)
	for _, block := range integrationBlocks(0, count) {
		blocks[block.Result.Number] = block
	}
	var mu sync.Mutex
	requested := make([]string, 0, count)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var rpcRequest struct {
			JSONRPC string            `json:"jsonrpc"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
			ID      int               `json:"id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&rpcRequest); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if rpcRequest.Method != "qrl_getBlockByNumber" || len(rpcRequest.Params) < 1 {
			http.Error(writer, "unexpected RPC method", http.StatusBadRequest)
			return
		}
		var number string
		if err := json.Unmarshal(rpcRequest.Params[0], &number); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		block, ok := blocks[number]
		if !ok {
			http.Error(writer, "unknown block", http.StatusNotFound)
			return
		}
		mu.Lock()
		requested = append(requested, number)
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(block); err != nil {
			t.Errorf("encode mock RPC block: %v", err)
		}
	}))
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), requested...)
	}
}

func runMigrationCommand(t *testing.T, workdir, mongoURI, nodeURL string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"run", "./cmd/reindex-block-companions"}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Dir = workdir
	cmd.Env = replaceEnvironment(os.Environ(), map[string]string{
		"MONGOURI":  mongoURI,
		"NODE_URL":  nodeURL,
		"NODE_URLS": "",
	})
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
	return string(output)
}

func assertLeaseReleased(t *testing.T, mongoURI string) {
	t.Helper()
	client := connectIntegrationMongo(t, mongoURI)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := client.Database("qrldata-z").Collection(configs.SYNCER_LEASES_COLLECTION).
		CountDocuments(ctx, bson.M{
			"_id":   "chain-indexer",
			"$expr": bson.M{"$lte": bson.A{"$expiresAt", "$$NOW"}},
		})
	if err != nil {
		t.Fatalf("check migration lease release: %v", err)
	}
	if count != 1 {
		t.Fatalf("released migration lease rows = %d, want 1", count)
	}
}
