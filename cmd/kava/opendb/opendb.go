//go:build !rocksdb
// +build !rocksdb

package opendb

import (
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/server/types"
	dbm "github.com/tendermint/tm-db"
)

func OpenDB(_ types.AppOptions, home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	return dbm.NewDB("application", backendType, dataDir)
}
