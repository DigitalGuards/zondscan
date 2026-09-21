package main

import (
	"QRL2MongoDB/db"
	"errors"
	"testing"
)

func TestProcessCandidatePagesResumesAtBudget(t *testing.T) {
	queue := []db.BlockCompanionReindexCandidate{
		{Number: "0x1"},
		{Number: "0x2"},
		{Number: "0x3"},
	}
	list := func(limit int) ([]db.BlockCompanionReindexCandidate, error) {
		if len(queue) < limit {
			limit = len(queue)
		}
		return append([]db.BlockCompanionReindexCandidate(nil), queue[:limit]...), nil
	}
	process := func(number string) error {
		if len(queue) == 0 || queue[0].Number != number {
			t.Fatalf("processed %s out of order from %v", number, queue)
		}
		queue = queue[1:]
		return nil
	}
	processed, next, err := processCandidatePages(2, 1, list, process)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 2 || next != "0x3" {
		t.Fatalf("processed, next = %d, %q, want 2, 0x3", processed, next)
	}
}

func TestProcessCandidatePagesRejectsRepeatedCandidate(t *testing.T) {
	listCalls := 0
	list := func(int) ([]db.BlockCompanionReindexCandidate, error) {
		listCalls++
		if listCalls > 2 {
			return nil, nil
		}
		return []db.BlockCompanionReindexCandidate{{Number: "0x1"}}, nil
	}
	processed, next, err := processCandidatePages(0, 1, list, func(string) error { return nil })
	if err == nil {
		t.Fatal("repeated candidate was accepted")
	}
	if processed != 1 || next != "0x1" {
		t.Fatalf("processed, next = %d, %q, want 1, 0x1", processed, next)
	}
}

func TestProcessCandidatePagesRejectsDecreasingCandidate(t *testing.T) {
	listCalls := 0
	list := func(int) ([]db.BlockCompanionReindexCandidate, error) {
		listCalls++
		if listCalls == 1 {
			return []db.BlockCompanionReindexCandidate{{Number: "0x2"}}, nil
		}
		if listCalls == 2 {
			return []db.BlockCompanionReindexCandidate{{Number: "0x1"}}, nil
		}
		return nil, nil
	}
	processed, next, err := processCandidatePages(0, 1, list, func(string) error { return nil })
	if err == nil {
		t.Fatal("decreasing candidate was accepted")
	}
	if processed != 1 || next != "0x1" {
		t.Fatalf("processed, next = %d, %q, want 1, 0x1", processed, next)
	}
}

func TestProcessCandidatePagesPropagatesListConflict(t *testing.T) {
	conflict := &db.BlockHeightConflictError{
		Number:   "0x2",
		Expected: "0xold",
		Found:    []string{"0xnew"},
	}
	_, _, err := processCandidatePages(0, 2,
		func(int) ([]db.BlockCompanionReindexCandidate, error) { return nil, conflict },
		func(string) error { return nil },
	)
	if !errors.Is(err, db.ErrBlockHeightConflict) {
		t.Fatalf("error = %v, want ErrBlockHeightConflict", err)
	}
}

func TestProcessMigrationWorkRepairsGapBeforeHigherMarkerlessBlock(t *testing.T) {
	candidates := []db.BlockCompanionReindexCandidate{
		{Number: "0x0"},
		{Number: "0x2"},
	}
	completed := make(map[string]bool)
	list := func(limit int) ([]db.BlockCompanionReindexCandidate, error) {
		remaining := make([]db.BlockCompanionReindexCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if !completed[candidate.Number] {
				remaining = append(remaining, candidate)
			}
		}
		if len(remaining) > limit {
			remaining = remaining[:limit]
		}
		return remaining, nil
	}
	processedOrder := make([]string, 0, 3)
	process := func(number string) error {
		switch number {
		case "0x0":
			completed[number] = true
		case "0x1":
			if !completed["0x0"] {
				return errors.New("missing completed genesis predecessor")
			}
			completed[number] = true
		case "0x2":
			if !completed["0x1"] {
				return errors.New("missing repaired predecessor 0x1")
			}
			completed[number] = true
		default:
			return errors.New("unexpected migration work height")
		}
		processedOrder = append(processedOrder, number)
		return nil
	}

	processed, next, err := processMigrationWork(
		0,
		2,
		list,
		func() ([]string, error) { return []string{"0x1"}, nil },
		process,
	)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 3 || next != "" {
		t.Fatalf("processed, next = %d, %q, want 3, empty", processed, next)
	}
	want := []string{"0x0", "0x1", "0x2"}
	for index := range want {
		if processedOrder[index] != want[index] {
			t.Fatalf("processed order = %v, want %v", processedOrder, want)
		}
	}
}
