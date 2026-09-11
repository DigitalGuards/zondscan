package configs

import "testing"

func TestMongoTopologySupportsTransactions(t *testing.T) {
	for _, test := range []struct {
		name    string
		setName string
		msg     string
		want    bool
	}{
		{name: "replica set", setName: "rs0", want: true},
		{name: "mongos", msg: "isdbgrid", want: true},
		{name: "standalone", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := mongoTopologySupportsTransactions(test.setName, test.msg); got != test.want {
				t.Fatalf("support = %v, want %v", got, test.want)
			}
		})
	}
}
