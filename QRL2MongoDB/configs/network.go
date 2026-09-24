package configs

import (
	"QRL2MongoDB/beaconprofile"
	"QRL2MongoDB/networkprofile"
	"QRL2MongoDB/validation"
	"context"
	"os"
)

// NativeAddressBytes describes the reviewed protocol implementation in this
// binary. It is deliberately independent of deployment environment variables.
const NativeAddressBytes = validation.AddressLength / 2

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
	if err := beaconprofile.Configure(ctx, os.Getenv, profile.Pinned); err != nil {
		return profile, err
	}
	return profile, nil
}
