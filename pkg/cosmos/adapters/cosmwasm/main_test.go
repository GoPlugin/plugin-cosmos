package cosmwasm

import (
	"os"
	"testing"

	"github.com/goplugin/plugin-cosmos/pkg/cosmos/params"
)

func TestMain(m *testing.M) {
	params.InitCosmosSdk(
		/* bech32Prefix= */ "wasm",
		/* token= */ "cosm",
	)
	code := m.Run()
	os.Exit(code)
}
