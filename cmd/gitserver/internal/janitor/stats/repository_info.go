package stats

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	// StaleObjectsGracePeriod is time delta that is used to indicate cutoff wherein an object
	// would be considered old. Currently this is set to being 10 days.
	StaleObjectsGracePeriod = -10 * 24 * time.Hour

	// fullRepackTimestampFilename is the name of the file that is used as a timestamp for the
	// last repack that happened in the repository. Whenever a full repack happens, we will
	// touch this file so that its last-modified date can be used to tell how long ago the last
	// full repack happened.
	fullRepackTimestampFilename = ".soucregraph-full-repack-timestamp"
)

// UpdateFullRepackTimestamp updates the timestamp that indicates when a repository has last been
// fully repacked to the current time.
func UpdateFullRepackTimestamp(repo common.GitDir, timestamp time.Time) (returnedErr error) {
	timestampPath := repo.Path(fullRepackTimestampFilename)

	// We first try to update an existing file so that we don't rewrite the file in case it
	// already exists, but only update its atime and mtime.
	if err := os.Chtimes(timestampPath, timestamp, timestamp); err == nil {
		// The file exists and we have successfully updated its timestamp. We can thus exit
		// early.
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		// Updating the timestamp failed, but not because the file doesn't exist. This
		// indicates an unexpected error and thus we return it to the caller.
		return err
	}

	// When the file does not yet exist we create it anew. Note that we do this via a temporary
	// file as it wouldn't otherwise be possible to atomically set the expected timestamp on it
	// because there is no API to create a file with a specific mtime.
	f, err := os.CreateTemp(repo.Path(), fullRepackTimestampFilename+"-*")
	if err != nil {
		return err
	}
	defer func() {
		// If the file still exists we try to remove it. We don't care for the error though:
		// when we have renamed the file into place it will fail, and that is expected. On
		// the other hand, if we didn't yet rename the file we know that we would already
		// return an error anyway. So in both cases, the error would be irrelevant.
		_ = os.Remove(f.Name())
	}()

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Chtimes(f.Name(), timestamp, timestamp); err != nil {
		return err
	}

	if err := os.Rename(f.Name(), timestampPath); err != nil {
		return err
	}

	return nil
}

// FullRepackTimestamp reads the timestamp that indicates when a repository has last been fully
// repacked. If no such timestamp exists, the zero timestamp is returned without an error.
func FullRepackTimestamp(repo common.GitDir) (time.Time, error) {
	stat, err := os.Stat(repo.Path(fullRepackTimestampFilename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// It's fine if the file doesn't exist. We just leave the timestamp at the
			// zero date in that case.
			return time.Time{}, nil
		}

		return time.Time{}, err
	}

	return stat.ModTime(), nil
}

// PackfilesCount returns the number of packfiles a repository has.
func PackfilesCount(repo common.GitDir) (uint64, error) {
	packfilesInfo, err := PackfilesInfoForRepository(repo)
	if err != nil {
		return 0, fmt.Errorf("deriving packfiles info: %w", err)
	}

	return packfilesInfo.Count, nil
}

// LooseObjects returns the number of loose objects that are not in a packfile.
func LooseObjects(repo common.GitDir) (uint64, error) {
	objectsInfo, err := LooseObjectsInfoForRepository(repo, time.Now())
	if err != nil {
		return 0, err
	}

	return objectsInfo.Count, nil
}

// RepositoryInfo contains information about the repository.
type RepositoryInfo struct {
	// LooseObjects contains information about loose objects.
	LooseObjects LooseObjectsInfo `json:"loose_objects"`
	// Packfiles contains information about packfiles.
	Packfiles PackfilesInfo `json:"packfiles"`
	// References contains information about the repository's references.
	References ReferencesInfo `json:"references"`
	// CommitGraph contains information about the repository's commit-graphs.
	CommitGraph CommitGraphInfo `json:"commit_graph"`
}

// RepositoryInfoForRepository computes the RepositoryInfo for a repository.
func RepositoryInfoForRepository(ctx context.Context, repo common.GitDir) (RepositoryInfo, error) {
	var info RepositoryInfo
	var err error

	info.LooseObjects, err = LooseObjectsInfoForRepository(repo, time.Now().Add(StaleObjectsGracePeriod))
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("counting loose objects: %w", err)
	}

	info.Packfiles, err = PackfilesInfoForRepository(repo)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("counting packfiles: %w", err)
	}

	info.References, err = ReferencesInfoForRepository(ctx, repo)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("checking references: %w", err)
	}

	info.CommitGraph, err = CommitGraphInfoForRepository(repo)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("checking commit-graph info: %w", err)
	}

	return info, nil
}

// ReferencesInfo contains information about references.
type ReferencesInfo struct {
	// LooseReferencesCount is the number of unpacked, loose references that exist.
	LooseReferencesCount uint64 `json:"loose_references_count"`
	// PackedReferencesSize is the size of the packed-refs file in bytes.
	PackedReferencesSize uint64 `json:"packed_references_size"`
	// ReftableTables contains details of individual table files.
	ReftableTables []ReftableTable `json:"reftable_tables"`
	// ReftableUnrecognizedFilesCount is the number of files under the `reftables/`
	// directory that shouldn't exist, according to the entries in `tables.list`.
	ReftableUnrecognizedFilesCount uint64 `json:"reftable_unrecognized_files"`
}

// ReftableTable contains information about an individual reftable table.
type ReftableTable struct {
	// Size is the size in bytes.
	Size uint64 `json:"size"`
	// UpdateIndexMin is the min_update_index of the reftable table. This is derived
	// from the filename only.
	UpdateIndexMin uint64 `json:"update_index_min"`
	// UpdateIndexMax is the max_update_index of the reftable table. This is derived
	// from the filename only.
	UpdateIndexMax uint64 `json:"update_index_max"`
}

// ReferencesInfoForRepository derives information about references in the repository.
func ReferencesInfoForRepository(ctx context.Context, repo common.GitDir) (ReferencesInfo, error) {
	var info ReferencesInfo

	refsPath := repo.Path("refs")
	if err := filepath.WalkDir(refsPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// It may happen that references got deleted concurrently. This is fine and expected, so we just
			// ignore any such errors.
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return err
		}

		if !entry.IsDir() {
			info.LooseReferencesCount++
		}

		return nil
	}); err != nil {
		return ReferencesInfo{}, fmt.Errorf("counting loose refs: %w", err)
	}

	if stat, err := os.Stat(repo.Path("packed-refs")); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return ReferencesInfo{}, fmt.Errorf("getting packed-refs size: %w", err)
		}
	} else {
		info.PackedReferencesSize = uint64(stat.Size())
	}

	return info, nil
}

// LooseObjectsInfo contains information about loose objects.
type LooseObjectsInfo struct {
	// Count is the number of loose objects.
	Count uint64 `json:"count"`
	// Size is the total size of all loose objects in bytes.
	Size uint64 `json:"size"`
	// StaleCount is the number of stale loose objects when taking into account the specified cutoff
	// date.
	StaleCount uint64 `json:"stale_count"`
	// StaleSize is the total size of stale loose objects when taking into account the specified
	// cutoff date.
	StaleSize uint64 `json:"stale_size"`
	// GarbageCount is the number of garbage files in the loose-objects shards.
	GarbageCount uint64 `json:"garbage_count"`
	// GarbageSize is the total size of garbage in the loose-objects shards.
	GarbageSize uint64 `json:"garbage_size"`
}

// LooseObjectsInfoForRepository derives information about loose objects in the repository. If a
// cutoff date is given, then this function will only take into account objects which are older than
// the given point in time.
func LooseObjectsInfoForRepository(repo common.GitDir, cutoffDate time.Time) (LooseObjectsInfo, error) {
	var info LooseObjectsInfo
	for i := 0; i <= 0xFF; i++ {
		entries, err := os.ReadDir(repo.Path("objects", fmt.Sprintf("%02x", i)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return LooseObjectsInfo{}, fmt.Errorf("reading loose object shard: %w", err)
		}

		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}

				return LooseObjectsInfo{}, fmt.Errorf("reading object info: %w", err)
			}

			if !isValidLooseObjectName(entry.Name()) {
				info.GarbageCount++
				info.GarbageSize += uint64(entryInfo.Size())
				continue
			}

			// Note: we don't `continue` here as we count stale objects into the total
			// number of objects.
			if entryInfo.ModTime().Before(cutoffDate) {
				info.StaleCount++
				info.StaleSize += uint64(entryInfo.Size())
			}

			info.Count++
			info.Size += uint64(entryInfo.Size())
		}
	}

	return info, nil
}

func isValidLooseObjectName(s string) bool {
	for _, c := range []byte(s) {
		if strings.IndexByte("0123456789abcdef", c) < 0 {
			return false
		}
	}
	return true
}

// PackfilesInfo contains information about packfiles.
type PackfilesInfo struct {
	// Count is the number of all packfiles, including stale and kept ones.
	Count uint64 `json:"count"`
	// Size is the total size of all packfiles in bytes, including stale and kept ones.
	Size uint64 `json:"size"`
	// ReverseIndexCount is the number of reverse indices.
	ReverseIndexCount uint64 `json:"reverse_index_count"`
	// CruftCount is the number of cruft packfiles which have a .mtimes file.
	CruftCount uint64 `json:"cruft_count"`
	// CruftSize is the size of cruft packfiles which have a .mtimes file.
	CruftSize uint64 `json:"cruft_size"`
	// KeepCount is the number of .keep packfiles.
	KeepCount uint64 `json:"keep_count"`
	// KeepSize is the size of .keep packfiles.
	KeepSize uint64 `json:"keep_size"`
	// GarbageCount is the number of garbage files.
	GarbageCount uint64 `json:"garbage_count"`
	// GarbageSize is the total size of all garbage files in bytes.
	GarbageSize uint64 `json:"garbage_size"`
	// Bitmap contains information about the bitmap, if any exists.
	Bitmap BitmapInfo `json:"bitmap"`
	// MultiPackIndex confains information about the multi-pack-index, if any exists.
	MultiPackIndex MultiPackIndexInfo `json:"multi_pack_index"`
	// MultiPackIndexBitmap contains information about the bitmap for the multi-pack-index, if
	// any exists.
	MultiPackIndexBitmap BitmapInfo `json:"multi_pack_index_bitmap"`
	// LastFullRepack indicates the last date at which a full repack has been performed. If the
	// date cannot be determined then this file is set to the zero time.
	LastFullRepack time.Time `json:"last_full_repack"`
}

// PackfilesInfoForRepository derives various information about packfiles for the given repository.
func PackfilesInfoForRepository(repo common.GitDir) (PackfilesInfo, error) {
	packfilesPath := repo.Path("objects", "pack")

	entries, err := os.ReadDir(packfilesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PackfilesInfo{}, nil
		}

		return PackfilesInfo{}, err
	}

	packfilesMetadata := classifyPackfiles(entries)

	var info PackfilesInfo
	for _, entry := range entries {
		entryName := entry.Name()

		switch {
		case hasPrefixAndSuffix(entryName, "pack-", ".pack"):
			size, err := entrySize(entry)
			if err != nil {
				return PackfilesInfo{}, fmt.Errorf("getting packfile size: %w", err)
			}

			info.Count++
			info.Size += size

			metadata := packfilesMetadata[entryName]
			switch {
			case metadata.hasKeep:
				info.KeepCount++
				info.KeepSize += size
			case metadata.hasMtimes:
				info.CruftCount++
				info.CruftSize += size
			}
		case hasPrefixAndSuffix(entryName, "pack-", ".idx"):
			// We ignore normal indices as every packfile would have one anyway, or
			// otherwise the repository would be corrupted.
		case hasPrefixAndSuffix(entryName, "pack-", ".keep"):
			// We classify .keep files above.
		case hasPrefixAndSuffix(entryName, "pack-", ".mtimes"):
			// We classify .mtimes files above.
		case hasPrefixAndSuffix(entryName, "pack-", ".rev"):
			info.ReverseIndexCount++
		case hasPrefixAndSuffix(entryName, "pack-", ".bitmap"):
			bitmap, err := BitmapInfoForPath(filepath.Join(packfilesPath, entryName))
			if err != nil {
				return PackfilesInfo{}, fmt.Errorf("reading bitmap info: %w", err)
			}

			info.Bitmap = bitmap
		case entryName == "multi-pack-index":
			midxInfo, err := MultiPackIndexInfoForPath(filepath.Join(packfilesPath, entryName))
			if err != nil {
				return PackfilesInfo{}, fmt.Errorf("reading multi-pack-index: %w", err)
			}

			info.MultiPackIndex = midxInfo
		case hasPrefixAndSuffix(entryName, "multi-pack-index-", ".bitmap"):
			bitmap, err := BitmapInfoForPath(filepath.Join(packfilesPath, entryName))
			if err != nil {
				return PackfilesInfo{}, fmt.Errorf("reading multi-pack-index bitmap info: %w", err)
			}

			info.MultiPackIndexBitmap = bitmap
		default:
			size, err := entrySize(entry)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// Unrecognized files may easily be temporary files written
					// by Git. It is expected that these may get concurrently
					// removed, so we just ignore the case where they've gone
					// missing.
					continue
				}

				return PackfilesInfo{}, fmt.Errorf("getting garbage size: %w", err)
			}

			info.GarbageCount++
			info.GarbageSize += size
		}
	}

	lastFullRepack, err := FullRepackTimestamp(repo)
	if err != nil {
		return PackfilesInfo{}, fmt.Errorf("reading full-repack timestamp: %w", err)
	}
	info.LastFullRepack = lastFullRepack

	return info, nil
}

type packfileMetadata struct {
	hasKeep, hasMtimes bool
}

// classifyPackfiles classifies all directory entries that look like packfiles and derives whether
// they have specific metadata or not. It returns a map of packfile names with the respective
// metadata that has been found.
func classifyPackfiles(entries []fs.DirEntry) map[string]packfileMetadata {
	packfileInfos := map[string]packfileMetadata{}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "pack-") {
			continue
		}

		extension := filepath.Ext(entry.Name())
		packfileName := strings.TrimSuffix(entry.Name(), extension) + ".pack"

		packfileMetadata := packfileInfos[packfileName]
		switch extension {
		case ".keep":
			packfileMetadata.hasKeep = true
		case ".mtimes":
			packfileMetadata.hasMtimes = true
		}
		packfileInfos[packfileName] = packfileMetadata
	}

	return packfileInfos
}

func entrySize(entry fs.DirEntry) (uint64, error) {
	entryInfo, err := entry.Info()
	if err != nil {
		return 0, fmt.Errorf("getting file info: %w", err)
	}

	if entryInfo.Size() >= 0 {
		return uint64(entryInfo.Size()), nil
	}

	return 0, nil
}

func hasPrefixAndSuffix(s, prefix, suffix string) bool {
	return strings.HasPrefix(s, prefix) && strings.HasSuffix(s, suffix)
}

// BitmapInfo contains information about a packfile or multi-pack-index bitmap.
type BitmapInfo struct {
	// Exists indicates whether the bitmap exists. This field would usually always be `true`
	// when read via `BitmapInfoForPath()`, but helps when the bitmap info is embedded into
	// another structure where it may only be conditionally read.
	Exists bool `json:"exists"`
	// Version is the version of the bitmap. Currently, this is expected to always be 1.
	Version uint16 `json:"version"`
	// HasHashCache indicates whether the name hash cache extension exists in the bitmap. This
	// extension records hashes of the path at which trees or blobs are found at the time of
	// writing the packfile so that it becomes possible to quickly find objects stored at the
	// same path. This mechanism is fed into the delta compression machinery to make the delta
	// heuristics more effective.
	HasHashCache bool `json:"has_hash_cache"`
	// HasLookupTable indicates whether the lookup table exists in the bitmap. Lookup tables
	// allow to defer loading bitmaps until required and thus speed up read-only bitmap
	// preparations.
	HasLookupTable bool `json:"has_lookup_table"`
}

// BitmapInfoForPath reads the bitmap at the given path and returns information on that bitmap.
func BitmapInfoForPath(path string) (BitmapInfo, error) {
	// The bitmap header is defined in
	// https://github.com/git/git/blob/master/Documentation/technical/bitmap-format.txt.
	bitmapHeader := []byte{
		0, 0, 0, 0, // 4-byte signature
		0, 0, // 2-byte version number in network byte order
		0, 0, // 2-byte flags in network byte order
	}

	file, err := os.Open(path)
	if err != nil {
		return BitmapInfo{}, fmt.Errorf("opening bitmap: %w", err)
	}
	defer file.Close()

	if _, err := io.ReadFull(file, bitmapHeader); err != nil {
		return BitmapInfo{}, fmt.Errorf("reading bitmap header: %w", err)
	}

	if !bytes.Equal(bitmapHeader[0:4], []byte{'B', 'I', 'T', 'M'}) {
		return BitmapInfo{}, fmt.Errorf("invalid bitmap signature: %q", string(bitmapHeader[0:4]))
	}

	version := binary.BigEndian.Uint16(bitmapHeader[4:6])
	if version != 1 {
		return BitmapInfo{}, fmt.Errorf("unsupported version: %d", version)
	}

	flags := binary.BigEndian.Uint16(bitmapHeader[6:8])

	return BitmapInfo{
		Exists:         true,
		Version:        version,
		HasHashCache:   flags&0x4 == 0x4,
		HasLookupTable: flags&0x10 == 0x10,
	}, nil
}

// MultiPackIndexInfo contains information about a multi-pack-index.
type MultiPackIndexInfo struct {
	// Exists determines whether the multi-pack-index exists or not.
	Exists bool `json:"exists"`
	// Version is the version of the multi-pack-index. Currently, Git only recognizes version 1.
	Version uint8 `json:"version"`
	// PackfileCount is the count of packfiles that the multi-pack-index tracks.
	PackfileCount uint64 `json:"packfile_count"`
}

// MultiPackIndexInfoForPath reads the multi-pack-index at the given path and returns information on
// it. Returns an error in case the file cannot be read or in case its format is not understood.
func MultiPackIndexInfoForPath(path string) (MultiPackIndexInfo, error) {
	// Please refer to gitformat-pack(5) for the definition of the multi-pack-index header.
	midxHeader := []byte{
		0, 0, 0, 0, // 4-byte signature
		0,          // 1-byte version number
		0,          // 1-byte object ID version
		0,          // 1-byte number of chunks
		0,          // 1-byte number of base multi-pack-index files
		0, 0, 0, 0, // 4-byte number of packfiles
	}

	file, err := os.Open(path)
	if err != nil {
		return MultiPackIndexInfo{}, fmt.Errorf("opening multi-pack-index: %w", err)
	}
	defer file.Close()

	if _, err := io.ReadFull(file, midxHeader); err != nil {
		return MultiPackIndexInfo{}, fmt.Errorf("reading header: %w", err)
	}

	if !bytes.Equal(midxHeader[0:4], []byte{'M', 'I', 'D', 'X'}) {
		return MultiPackIndexInfo{}, fmt.Errorf("invalid signature: %q", string(midxHeader[0:4]))
	}

	version := midxHeader[4]
	if version != 1 {
		return MultiPackIndexInfo{}, fmt.Errorf("invalid version: %d", version)
	}

	baseFiles := midxHeader[7]
	if baseFiles != 0 {
		return MultiPackIndexInfo{}, fmt.Errorf("unsupported number of base files: %d", baseFiles)
	}

	packfileCount := binary.BigEndian.Uint32(midxHeader[8:12])

	return MultiPackIndexInfo{
		Exists:        true,
		Version:       version,
		PackfileCount: uint64(packfileCount),
	}, nil
}
