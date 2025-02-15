/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenFileLinks(t *testing.T) {
	// symlink structure setup, this will produce the following structure under the temp directory created below:
	// dir
	// dir-s -> dir
	// dir-s-s -> dir-s
	// dir/file
	// dir/file-s -> dir/file
	// circular-s -> circular-s
	// broken-s -> nonexistent
	// hardfile
	// hardfile-h -> hardfile

	// macOS is special cased here since t.TempDir() returns a path to a protected directory that doesn't allow symlinks.
	var rootDir string
	var err error
	switch runtime.GOOS {
	case "darwin":
		rootDir, err = os.MkdirTemp("/private/tmp", "teleport-test-*")
		require.NoError(t, err)

		t.Cleanup(func() {
			err := os.RemoveAll(rootDir)
			require.NoError(t, err)
		})
	default:
		rootDir = t.TempDir()
	}

	dirPath := filepath.Join(rootDir, "dir")
	err = os.Mkdir(dirPath, 0755)
	require.NoError(t, err)

	dirFilePath := filepath.Join(dirPath, "file")
	f, err := os.Create(dirFilePath)
	require.NoError(t, err)
	f.Close()

	dirSymlinkToFile := filepath.Join(dirPath, "file-s")
	err = os.Symlink(dirFilePath, dirSymlinkToFile)
	require.NoError(t, err)

	symlinkDir := filepath.Join(rootDir, "dir-s")
	err = os.Symlink(dirPath, symlinkDir)
	require.NoError(t, err)

	symlinkToSymlinkDir := filepath.Join(rootDir, "dir-s-s")
	err = os.Symlink(symlinkDir, symlinkToSymlinkDir)
	require.NoError(t, err)

	circularSymlink := filepath.Join(rootDir, "circular-s")
	err = os.Symlink(circularSymlink, circularSymlink)
	require.NoError(t, err)

	brokenSymlink := filepath.Join(rootDir, "broken-s")
	err = os.Symlink(filepath.Join(rootDir, "nonexistent"), brokenSymlink)
	require.NoError(t, err)

	dirHardfilePath := filepath.Join(rootDir, "hardfile")
	f, err = os.Create(dirHardfilePath)
	require.NoError(t, err)
	f.Close()

	dirHardLinkToHardfile := filepath.Join(rootDir, "hardfile-h")
	err = os.Link(dirHardfilePath, dirHardLinkToHardfile)
	require.NoError(t, err)

	// Define and run tests against underline openFile function
	type openFileTestCase struct {
		name        string
		filePath    string
		allowSymln  bool
		allowHardln bool
		expectErr   string
	}
	commonOpenFileTestCases := []openFileTestCase{
		{
			name:        "directFileOpenAllowed",
			filePath:    dirFilePath,
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "",
		},
		{
			name:        "symlinkFileOpenAllowed",
			filePath:    dirSymlinkToFile,
			allowSymln:  true,
			allowHardln: false,
			expectErr:   "",
		},
		{
			name:        "hardlinkFileOpenAllowed",
			filePath:    dirHardLinkToHardfile,
			allowSymln:  false,
			allowHardln: true,
			expectErr:   "",
		},
		{
			name:        "symlinkFileOpenDenied",
			filePath:    dirSymlinkToFile,
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "symlink",
		},
		{
			name:        "symlinkDirFileOpenDenied",
			filePath:    filepath.Join(symlinkDir, "file"),
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "symlink",
		},
		{
			name:        "symlinkRecursiveDirFileOpenDenied",
			filePath:    filepath.Join(symlinkToSymlinkDir, "file"),
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "symlink",
		},
		{
			name:        "hardlinkFileOpenDenied",
			filePath:    dirHardLinkToHardfile,
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "hardlink",
		},
	}
	openFileTestCases := append(commonOpenFileTestCases, []openFileTestCase{
		{
			name:        "circularSymlink",
			filePath:    circularSymlink,
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "symlink",
		},
		{
			name:        "brokenSymlink",
			filePath:    brokenSymlink,
			allowSymln:  false,
			allowHardln: false,
			expectErr:   "symlink",
		},
	}...)

	for _, tt := range openFileTestCases {
		t.Run(tt.name, func(t *testing.T) {
			f, err := openFile(tt.filePath, tt.allowSymln, tt.allowHardln)
			if f != nil {
				f.Close()
			}
			if tt.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectErr)
			}
		})
	}

	// Define and run tests against OS specific public functions
	// OpenFileAllowingUnsafeLinks should always allow all the common test cases to pass without error
	for _, tt := range commonOpenFileTestCases {
		t.Run("unsafe-"+tt.name, func(t *testing.T) {
			f, err := OpenFileAllowingUnsafeLinks(tt.filePath)
			if f != nil {
				f.Close()
			}
			require.NoError(t, err)
		})
	}
	// OpenFileNoUnsafeLinks has OS conditional logic that necessitates us to define the expected behavior
	type safeOpenFileTestCase struct {
		name      string
		filePath  string
		expectErr string
	}
	safeOpenFileTestCases := []safeOpenFileTestCase{
		{
			name:      "directFileOpenAllowed",
			filePath:  dirFilePath,
			expectErr: "",
		},
		{
			name:      "symlinkFileOpenDenied",
			filePath:  dirSymlinkToFile,
			expectErr: "symlink",
		},
		{
			name:      "symlinkDirFileOpenDenied",
			filePath:  filepath.Join(symlinkDir, "file"),
			expectErr: "symlink",
		},
		{
			name:      "symlinkRecursiveDirFileOpenDenied",
			filePath:  filepath.Join(symlinkToSymlinkDir, "file"),
			expectErr: "symlink",
		},
		{
			name:      "circularSymlink",
			filePath:  circularSymlink,
			expectErr: "symlink",
		},
		{
			name:      "brokenSymlink",
			filePath:  brokenSymlink,
			expectErr: "symlink",
		},
	}
	if runtime.GOOS == "darwin" {
		safeOpenFileTestCases = append(safeOpenFileTestCases, safeOpenFileTestCase{
			name:      "hardlinkFileOpen",
			filePath:  dirHardLinkToHardfile,
			expectErr: "hardlink",
		})
	} else {
		safeOpenFileTestCases = append(safeOpenFileTestCases, safeOpenFileTestCase{
			name:      "hardlinkFileOpen",
			filePath:  dirHardLinkToHardfile,
			expectErr: "",
		})
	}
	for _, tt := range safeOpenFileTestCases {
		t.Run("safe-"+tt.name, func(t *testing.T) {
			f, err := OpenFileNoUnsafeLinks(tt.filePath)
			if f != nil {
				f.Close()
			}
			if tt.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectErr)
			}
		})
	}
}

func TestLocks(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "teleport-lock-test")
	fp := tmpFile.Name()
	t.Cleanup(func() {
		_ = os.Remove(fp)
	})
	require.NoError(t, err)

	// Can take read lock
	unlock, err := FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())

	// Can take write lock
	unlock, err = FSTryWriteLock(fp)
	require.NoError(t, err)

	// Can't take read lock while write lock is held.
	unlock2, err := FSTryReadLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	// Can't take write lock while another write lock is held.
	unlock2, err = FSTryWriteLock(fp)
	require.ErrorIs(t, err, ErrUnsuccessfulLockTry)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())

	unlock, err = FSTryReadLock(fp)
	require.NoError(t, err)

	// Can take second read lock on the same file.
	unlock2, err = FSTryReadLock(fp)
	require.NoError(t, err)

	require.NoError(t, unlock())
	require.NoError(t, unlock2())

	// Can take read lock with timeout
	unlock, err = FSTryReadLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)
	require.NoError(t, unlock())

	// Can take write lock with timeout
	unlock, err = FSTryWriteLockTimeout(context.Background(), fp, time.Second)
	require.NoError(t, err)

	// Fails because timeout is exceeded, since file is already locked.
	unlock2, err = FSTryWriteLockTimeout(context.Background(), fp, time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	// Fails because context is expired while waiting for timeout.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	unlock2, err = FSTryWriteLockTimeout(ctx, fp, time.Hour*1000)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, unlock2)

	require.NoError(t, unlock())
}

func TestOverwriteFile(t *testing.T) {
	have := []byte("Sensitive Information")
	fName := filepath.Join(t.TempDir(), "teleport-overwrite-file-test")

	require.NoError(t, os.WriteFile(fName, have, 0600))
	f, err := os.OpenFile(fName, os.O_WRONLY, 0)
	require.NoError(t, err)
	defer f.Close()
	fi, err := os.Stat(fName)
	require.NoError(t, err)
	require.NoError(t, overwriteFile(f, fi))

	contents, err := os.ReadFile(fName)
	require.NoError(t, err)
	require.NotContains(t, contents, have, "File contents were not overwritten")
}

func TestRemoveAllSecure(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "teleport-remove-all-secure-test")
	f, err := os.Create(tempFile)
	symlink := filepath.Join(tempDir, "teleport-remove-secure-symlink")
	require.NoError(t, os.Symlink(tempFile, symlink))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, RemoveAllSecure(""))
	require.NoError(t, RemoveAllSecure(tempDir))
	_, err = os.Stat(tempDir)
	require.True(t, os.IsNotExist(err), "Directory should be removed: %v", err)
}

func TestRemoveSecure(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "teleport-remove-secure-test")
	f, err := os.Create(tempFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, RemoveSecure(f.Name()))
	_, err = os.Stat(tempFile)
	require.True(t, os.IsNotExist(err), "File should be removed: %v", err)
}

func TestRemoveSecure_symlink(t *testing.T) {
	symlink := filepath.Join(t.TempDir(), "teleport-remove-secure-symlink")
	require.NoError(t, os.Symlink("/tmp", symlink))

	require.NoError(t, RemoveSecure(symlink))
	_, err := os.Stat(symlink)
	require.True(t, os.IsNotExist(err), "Symlink should be removed: %v", err)
}
