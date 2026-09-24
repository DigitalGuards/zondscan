package networkprofile

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// This integration test only accepts an explicit loopback test server and
// creates uniquely named databases. It never uses MONGOURI or qrldata-z.
func TestMongoNetworkIsolation(t *testing.T) {
	uri := os.Getenv("NETWORK_TEST_MONGOURI")
	if uri == "" {
		t.Skip("set NETWORK_TEST_MONGOURI to a disposable loopback MongoDB")
	}
	parsed, err := url.Parse(uri)
	if err != nil || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" && parsed.Hostname() != "::1") {
		t.Fatal("NETWORK_TEST_MONGOURI must be loopback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("network_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	makeDB := func(suffix string) *mongo.Database {
		database := client.Database(prefix + suffix)
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := database.Drop(cleanupCtx); err != nil {
				t.Error(err)
			}
		})
		return database
	}
	legacy, future := makeDB("_v2"), makeDB("_v3")
	v2 := Profile{Identity: Identity{"v2", "", "", 20, false}, DatabaseName: legacy.Name()}
	v3 := Profile{Identity: Identity{"v3", "1337", testGenesis, 64, true}, DatabaseName: future.Name(), Pinned: true}
	if _, err := legacy.Collection("transfer").InsertOne(ctx, bson.M{"_id": "v2-sentinel", "value": "preserved"}); err != nil {
		t.Fatal(err)
	}
	if err := Bind(ctx, legacy, v2); err != nil {
		t.Fatal(err)
	}
	if err := Bind(ctx, future, v3); err != nil {
		t.Fatal(err)
	}
	if _, err := future.Collection("transfer").InsertOne(ctx, bson.M{"_id": "v3-sentinel"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		database *mongo.Database
		profile  Profile
	}{
		{"v3 against v2", legacy, Profile{Identity: v3.Identity, DatabaseName: legacy.Name(), Pinned: true}},
		{"v2 against v3", future, Profile{Identity: v2.Identity, DatabaseName: future.Name()}},
		{"changed genesis", future, Profile{Identity: Identity{"v3", "1337", "0x" + strings.Repeat("2", 64), 64, true}, DatabaseName: future.Name(), Pinned: true}},
		{"pinning unverified marker", legacy, Profile{Identity: Identity{"v2", "1337", testGenesis, 20, true}, DatabaseName: legacy.Name(), Pinned: true}},
		{"wrong selected database", future, v2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Bind(ctx, tc.database, tc.profile); err == nil {
				t.Fatal("accepted identity mismatch")
			}
		})
	}
	var sentinel bson.M
	if err := legacy.Collection("transfer").FindOne(ctx, bson.M{"_id": "v2-sentinel"}).Decode(&sentinel); err != nil || sentinel["value"] != "preserved" {
		t.Fatalf("v2 changed: %+v %v", sentinel, err)
	}
	if count, err := legacy.Collection("transfer").CountDocuments(ctx, bson.D{}); err != nil || count != 1 {
		t.Fatal("v3 wrote v2 database", count, err)
	}
	unmarked := makeDB("_unmarked")
	if _, err := unmarked.Collection("blocks").InsertOne(ctx, bson.M{"sentinel": true}); err != nil {
		t.Fatal(err)
	}
	unmarkedProfile := v3
	unmarkedProfile.DatabaseName = unmarked.Name()
	if err := Bind(ctx, unmarked, unmarkedProfile); err == nil {
		t.Fatal("adopted unmarked nonempty v3 database")
	}
	if count, _ := unmarked.Collection(IdentityCollection).CountDocuments(ctx, bson.D{}); count != 0 {
		t.Fatal("rejected adoption wrote identity")
	}
	storedGenesis := makeDB("_wrong_genesis")
	if _, err := storedGenesis.Collection("blocks").InsertOne(ctx, bson.M{"blockNumberInt": int64(0), "result": bson.M{"hash": "0x" + strings.Repeat("2", 64)}}); err != nil {
		t.Fatal(err)
	}
	pinnedV2 := Profile{Identity: Identity{"v2", "1337", testGenesis, 20, true}, DatabaseName: storedGenesis.Name(), Pinned: true}
	if err := Bind(ctx, storedGenesis, pinnedV2); err == nil {
		t.Fatal("adopted v2 data from wrong genesis")
	}
	missingGenesis := makeDB("_missing_genesis")
	if _, err := missingGenesis.Collection("transfer").InsertOne(ctx, bson.M{"sentinel": true}); err != nil {
		t.Fatal(err)
	}
	pinnedV2.DatabaseName = missingGenesis.Name()
	if err := Bind(ctx, missingGenesis, pinnedV2); err == nil {
		t.Fatal("adopted populated pinned database without genesis evidence")
	}
	malformed := makeDB("_malformed_marker")
	if _, err := malformed.Collection(IdentityCollection).InsertOne(ctx, bson.M{"_id": identityKey, "networkId": "v2", "chainId": "", "genesisHash": "", "addressBytes": 20}); err != nil {
		t.Fatal(err)
	}
	malformedProfile := v2
	malformedProfile.DatabaseName = malformed.Name()
	if err := Bind(ctx, malformed, malformedProfile); err == nil {
		t.Fatal("accepted marker without explicit verification state")
	}
	concurrent := makeDB("_concurrent")
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := v3
			p.DatabaseName = concurrent.Name()
			failures <- Bind(ctx, concurrent, p)
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal("concurrent identical binding", err)
		}
	}
	if count, err := concurrent.Collection(IdentityCollection).CountDocuments(ctx, bson.D{}); err != nil || count != 1 {
		t.Fatal("identity singleton not preserved", count, err)
	}
}
