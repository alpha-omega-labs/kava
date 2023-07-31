//go:build rocksdb
// +build rocksdb

// Copyright 2023 Kava Labs, Inc.
// Copyright 2023 Cronos Labs, Inc.
//
// Derived from https://github.com/crypto-org-chain/cronos@496ce7e
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opendb

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/linxGnu/grocksdb"
	dbm "github.com/tendermint/tm-db"
)

const BlockCacheSize = 1 << 30

func OpenDB(_ types.AppOptions, home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	if backendType == dbm.RocksDBBackend {
		return openRocksdb(filepath.Join(dataDir, "application.db"))
	}

	return dbm.NewDB("application", backendType, dataDir)
}

func openRocksdb(dir string) (dbm.DB, error) {
	opts, err := loadLatestOptions(dir)
	if err != nil {
		return nil, err
	}
	// customize rocksdb options
	opts = NewRocksdbOptions(opts, false)

	// func NewRocksDBWithOptions(name string, dir string, opts *grocksdb.Options) (*RocksDB, error) {
	return dbm.NewRocksDBWithOptions("application", dir, opts)
}

// loadLatestOptions try to load options from existing db, returns nil if not exists.
func loadLatestOptions(dir string) (*grocksdb.Options, error) {
	opts, err := grocksdb.LoadLatestOptions(dir, grocksdb.NewDefaultEnv(), true, grocksdb.NewLRUCache(BlockCacheSize))
	if err != nil {
		// not found is not an error
		if strings.HasPrefix(err.Error(), "NotFound: ") {
			return nil, nil
		}
		return nil, err
	}

	cfNames := opts.ColumnFamilyNames()
	cfOpts := opts.ColumnFamilyOpts()

	for i := 0; i < len(cfNames); i++ {
		if cfNames[i] == "default" {
			return &cfOpts[i], nil
		}
	}

	return opts.Options(), nil
}

// NewRocksdbOptions build options for `application.db`,
// it overrides existing options if provided, otherwise create new one assuming it's a new database.
func NewRocksdbOptions(opts *grocksdb.Options, sstFileWriter bool) *grocksdb.Options {
	if opts == nil {
		opts = grocksdb.NewDefaultOptions()
		// only enable dynamic-level-bytes on new db, don't override for existing db
		opts.SetLevelCompactionDynamicLevelBytes(true)
	}
	opts.SetCreateIfMissing(true)
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()

	// 1G block cache
	bbto.SetBlockCache(grocksdb.NewLRUCache(BlockCacheSize))

	// http://rocksdb.org/blog/2021/12/29/ribbon-filter.html
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))

	// partition index
	// http://rocksdb.org/blog/2017/05/12/partitioned-index-filter.html
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)
	bbto.SetOptimizeFiltersForMemory(true)

	// hash index is better for iavl tree which mostly do point lookup.
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)

	opts.SetBlockBasedTableFactory(bbto)

	// in iavl tree, we almost always query existing keys
	opts.SetOptimizeFiltersForHits(true)

	// heavier compression option at bottommost level,
	// 110k dict bytes is default in zstd library,
	// train bytes is recommended to be set at 100x dict bytes.
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	compressOpts.Level = 12
	if !sstFileWriter {
		compressOpts.MaxDictBytes = 110 * 1024
		opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	}
	opts.SetBottommostCompressionOptions(compressOpts, true)

	return opts
}
