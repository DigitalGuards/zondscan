package main

import (
	"QRL2MongoDB/configs"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

type recoveryInput struct {
	owner         string
	generation    int64
	computeFenced bool
}

func validateRecoveryInput(input recoveryInput) error {
	if !input.computeFenced {
		return errors.New("--compute-fenced is required after proving the old process or host cannot execute")
	}
	if input.owner == "" {
		return errors.New("--owner is required and must match the inspected lease row")
	}
	if input.generation <= 0 {
		return errors.New("--generation must be a positive value from the inspected lease row")
	}
	return nil
}

func run(args []string) error {
	flags := flag.NewFlagSet("recover-syncer-lease", flag.ContinueOnError)
	input := recoveryInput{}
	flags.StringVar(&input.owner, "owner", "", "exact owner from the expired chain-indexer lease")
	flags.Int64Var(&input.generation, "generation", 0, "exact generation from the expired chain-indexer lease")
	flags.BoolVar(
		&input.computeFenced,
		"compute-fenced",
		false,
		"confirm the old process and host cannot execute again",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if err := validateRecoveryInput(input); err != nil {
		return err
	}

	configs.LoadEnv()
	if err := configs.ConnectDB(); err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = configs.DB.Disconnect(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := configs.AcknowledgeExpiredSyncerLeaseAfterComputeFence(
		ctx,
		configs.SyncerLease{
			Owner:      input.owner,
			Generation: input.generation,
		},
	); err != nil {
		return fmt.Errorf("acknowledge expired syncer lease: %w", err)
	}
	fmt.Printf("expired chain-indexer lease generation %d acknowledged after compute fencing\n",
		input.generation)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
