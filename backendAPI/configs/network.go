package configs

import (
	"backendAPI/networkprofile"
	"context"
	"os"
)

// NativeAddressBytes describes the reviewed protocol implementation in this
// binary. It is deliberately independent of deployment environment variables.
const NativeAddressBytes = 20

var activeNetwork = networkprofile.Profile{
	Identity:     networkprofile.Identity{NetworkID: "v2", AddressBytes: NativeAddressBytes},
	DatabaseName: "qrldata-z",
}

func DatabaseName() string { return activeNetwork.DatabaseName }

func NetworkIdentity() networkprofile.Identity { return activeNetwork.Identity }

func configuredNetwork(ctx context.Context) (networkprofile.Profile, error) {
	profile, err := networkprofile.Parse(os.Getenv, NativeAddressBytes)
	if err != nil {
		return profile, err
	}
	if err := networkprofile.ValidateSources(ctx, profile, networkprofile.Sources(os.Getenv)); err != nil {
		return profile, err
	}
	return profile, nil
}
