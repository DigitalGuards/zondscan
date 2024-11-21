package db

import (
	"testing"
	"time"
)

func TestDateTime(t *testing.T) {
	currentTime := time.Now()
	got := ReturnDateTime()
	want := currentTime.Format("2006-01-02 3:4:5 PM")

	if got != want {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

// func TestCountWallets(t *testing.T){
// 	got :=
// 	want :=
// }
