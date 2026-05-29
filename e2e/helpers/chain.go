// Package helpers provides interchaintest chain specs for the Vertix e2e suite.
package helpers

import (
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// Image is the locally-built vertixd image (see repo-root Dockerfile).
// Override Version via the VERTIX_IMAGE_TAG env when running in CI.
const (
	ImageRepository = "vertix-network/vertixd"
	ImageVersion    = "local"
)

// VertixChainSpec returns an interchaintest ChainSpec for a single Vertix chain.
// chainID disambiguates the two chains in a Vertix<->Vertix test.
func VertixChainSpec(chainID string, nv, nf int) *interchaintest.ChainSpec {
	return &interchaintest.ChainSpec{
		Name:          "vertix",
		ChainName:     chainID,
		Version:       ImageVersion,
		NumValidators: &nv,
		NumFullNodes:  &nf,
		ChainConfig: ibc.ChainConfig{
			Type:           "cosmos",
			Name:           "vertix",
			ChainID:        chainID,
			Bin:            "vertixd",
			Bech32Prefix:   "vtx",
			Denom:          "uvtx",
			CoinType:       "118",
			GasPrices:      "0.025uvtx",
			GasAdjustment:  1.3,
			TrustingPeriod: "336h",
			Images: []ibc.DockerImage{
				{Repository: ImageRepository, Version: ImageVersion, UidGid: "1025:1025"},
			},
		},
	}
}
