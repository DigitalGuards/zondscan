package main

import (
	"strings"
	"testing"
)

func TestValidateRecoveryInputRequiresComputeFenceAndExactIdentity(t *testing.T) {
	tests := []struct {
		name  string
		input recoveryInput
		want  string
	}{
		{
			name:  "compute fence",
			input: recoveryInput{owner: "owner", generation: 1},
			want:  "--compute-fenced",
		},
		{
			name:  "owner",
			input: recoveryInput{generation: 1, computeFenced: true},
			want:  "--owner",
		},
		{
			name:  "generation",
			input: recoveryInput{owner: "owner", computeFenced: true},
			want:  "--generation",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRecoveryInput(test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateRecoveryInputAcceptsComputeFencedIdentity(t *testing.T) {
	if err := validateRecoveryInput(recoveryInput{
		owner:         "owner",
		generation:    7,
		computeFenced: true,
	}); err != nil {
		t.Fatal(err)
	}
}
