package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
)

// Try all of the test cases from the archive package which implements the
// internals of `docker cp` and ensure that the behavior matches when actually
// copying to and from containers.

// Basic assumptions about SRC and DST:
// 1. SRC must exist.
// 2. If SRC ends with a trailing separator, it must be a directory.
// 3. DST parent directory must exist.
// 4. If DST exists as a file, it must not end with a trailing separator.

// Check that copying from a container to a local symlink copies to the symlink
// target and does not overwrite the local symlink itself.
// TODO: move to docker/cli and/or integration/container/copy_test.go
func (s *DockerSuite) TestCpFromSymlinkDestination(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{addContent: true})

	tmpDir := getTestDir(c, "test-cp-from-err-dst-not-dir")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	// First, copy a file from the container to a symlink to a file. This
	// should overwrite the symlink target contents with the source contents.
	srcPath := containerCpPath(containerID, "/file2")
	dstPath := cpPath(tmpDir, "symlinkToFile1")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	// The symlink should not have been modified.
	assert.Assert(c, symlinkTargetEquals(c, dstPath, "file1") == nil)

	// The file should have the contents of "file2" now.
	assert.Assert(c, fileContentEquals(c, cpPath(tmpDir, "file1"), "file2\n") == nil)

	// Next, copy a file from the container to a symlink to a directory. This
	// should copy the file into the symlink target directory.
	dstPath = cpPath(tmpDir, "symlinkToDir1")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	// The symlink should not have been modified.
	assert.Assert(c, symlinkTargetEquals(c, dstPath, "dir1") == nil)

	// The file should have the contents of "file2" now.
	assert.Assert(c, fileContentEquals(c, cpPath(tmpDir, "file2"), "file2\n") == nil)

	// Next, copy a file from the container to a symlink to a file that does
	// not exist (a broken symlink). This should create the target file with
	// the contents of the source file.
	dstPath = cpPath(tmpDir, "brokenSymlinkToFileX")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	// The symlink should not have been modified.
	assert.Assert(c, symlinkTargetEquals(c, dstPath, "fileX") == nil)

	// The file should have the contents of "file2" now.
	assert.Assert(c, fileContentEquals(c, cpPath(tmpDir, "fileX"), "file2\n") == nil)

	// Next, copy a directory from the container to a symlink to a local
	// directory. This should copy the directory into the symlink target
	// directory and not modify the symlink.
	srcPath = containerCpPath(containerID, "/dir2")
	dstPath = cpPath(tmpDir, "symlinkToDir1")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	// The symlink should not have been modified.
	assert.Assert(c, symlinkTargetEquals(c, dstPath, "dir1") == nil)

	// The directory should now contain a copy of "dir2".
	assert.Assert(c, fileContentEquals(c, cpPath(tmpDir, "dir1/dir2/file2-1"), "file2-1\n") == nil)

	// Next, copy a directory from the container to a symlink to a local
	// directory that does not exist (a broken symlink). This should create
	// the target as a directory with the contents of the source directory. It
	// should not modify the symlink.
	dstPath = cpPath(tmpDir, "brokenSymlinkToDirX")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	// The symlink should not have been modified.
	assert.Assert(c, symlinkTargetEquals(c, dstPath, "dirX") == nil)

	// The "dirX" directory should now be a copy of "dir2".
	assert.Assert(c, fileContentEquals(c, cpPath(tmpDir, "dirX/file2-1"), "file2-1\n") == nil)
}

// Possibilities are reduced to the remaining 10 cases:
//
//  case | srcIsDir | onlyDirContents | dstExists | dstIsDir | dstTrSep | action
// ===================================================================================================
//   A   |  no      |  -              |  no       |  -       |  no      |  create file
//   B   |  no      |  -              |  no       |  -       |  yes     |  error
//   C   |  no      |  -              |  yes      |  no      |  -       |  overwrite file
//   D   |  no      |  -              |  yes      |  yes     |  -       |  create file in dst dir
//   E   |  yes     |  no             |  no       |  -       |  -       |  create dir, copy contents
//   F   |  yes     |  no             |  yes      |  no      |  -       |  error
//   G   |  yes     |  no             |  yes      |  yes     |  -       |  copy dir and contents
//   H   |  yes     |  yes            |  no       |  -       |  -       |  create dir, copy contents
//   I   |  yes     |  yes            |  yes      |  no      |  -       |  error
//   J   |  yes     |  yes            |  yes      |  yes     |  -       |  copy dir contents
//

// A. SRC specifies a file and DST (no trailing path separator) doesn't
//    exist. This should create a file with the name DST and copy the
//    contents of the source file into it.
func (s *DockerSuite) TestCpFromCaseA(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-a")
	defer os.RemoveAll(tmpDir)

	srcPath := containerCpPath(containerID, "/root/file1")
	dstPath := cpPath(tmpDir, "itWorks.txt")

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1\n") == nil)
}

// B. SRC specifies a file and DST (with trailing path separator) doesn't
//    exist. This should cause an error because the copy operation cannot
//    create a directory when copying a single file.
func (s *DockerSuite) TestCpFromCaseB(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{addContent: true})

	tmpDir := getTestDir(c, "test-cp-from-case-b")
	defer os.RemoveAll(tmpDir)

	srcPath := containerCpPath(containerID, "/file1")
	dstDir := cpPathTrailingSep(tmpDir, "testDir")

	err := runDockerCp(c, srcPath, dstDir, nil)
	assert.ErrorContains(c, err, "")

	assert.Assert(c, isCpDirNotExist(err), fmt.Sprintf("expected DirNotExists error, but got %T: %s", err, err))
}

// C. SRC specifies a file and DST exists as a file. This should overwrite
//    the file at DST with the contents of the source file.
func (s *DockerSuite) TestCpFromCaseC(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-c")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := containerCpPath(containerID, "/root/file1")
	dstPath := cpPath(tmpDir, "file2")

	// Ensure the local file starts with different content.
	assert.Assert(c, fileContentEquals(c, dstPath, "file2\n") == nil)

	assert.Assert(c, runDockerCp(c, srcPath, dstPath, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1\n") == nil)
}

// D. SRC specifies a file and DST exists as a directory. This should place
//    a copy of the source file inside it using the basename from SRC. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCpFromCaseD(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{addContent: true})

	tmpDir := getTestDir(c, "test-cp-from-case-d")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcPath := containerCpPath(containerID, "/file1")
	dstDir := cpPath(tmpDir, "dir1")
	dstPath := filepath.Join(dstDir, "file1")

	// Ensure that dstPath doesn't exist.
	_, err := os.Stat(dstPath)
	assert.Assert(c, os.IsNotExist(err), fmt.Sprintf("did not expect dstPath %q to exist", dstPath))

	assert.Assert(c, runDockerCp(c, srcPath, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1\n") == nil)

	// Now try again but using a trailing path separator for dstDir.

	// unable to remove dstDir
	assert.Assert(c, os.RemoveAll(dstDir) == nil)

	// unable to make dstDir
	assert.Assert(c, os.MkdirAll(dstDir, os.FileMode(0755)) == nil)

	dstDir = cpPathTrailingSep(tmpDir, "dir1")

	assert.Assert(c, runDockerCp(c, srcPath, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1\n") == nil)
}

// E. SRC specifies a directory and DST does not exist. This should create a
//    directory at DST and copy the contents of the SRC directory into the DST
//    directory. Ensure this works whether DST has a trailing path separator or
//    not.
func (s *DockerSuite) TestCpFromCaseE(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{addContent: true})

	tmpDir := getTestDir(c, "test-cp-from-case-e")
	defer os.RemoveAll(tmpDir)

	srcDir := containerCpPath(containerID, "dir1")
	dstDir := cpPath(tmpDir, "testDir")
	dstPath := filepath.Join(dstDir, "file1-1")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)

	// Now try again but using a trailing path separator for dstDir.

	// unable to remove dstDir
	assert.Assert(c, os.RemoveAll(dstDir) == nil)

	dstDir = cpPathTrailingSep(tmpDir, "testDir")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)
}

// F. SRC specifies a directory and DST exists as a file. This should cause an
//    error as it is not possible to overwrite a file with a directory.
func (s *DockerSuite) TestCpFromCaseF(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-f")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := containerCpPath(containerID, "/root/dir1")
	dstFile := cpPath(tmpDir, "file1")

	err := runDockerCp(c, srcDir, dstFile, nil)
	assert.ErrorContains(c, err, "")

	assert.Assert(c, isCpCannotCopyDir(err), fmt.Sprintf("expected ErrCannotCopyDir error, but got %T: %s", err, err))
}

// G. SRC specifies a directory and DST exists as a directory. This should copy
//    the SRC directory and all its contents to the DST directory. Ensure this
//    works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCpFromCaseG(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-g")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := containerCpPath(containerID, "/root/dir1")
	dstDir := cpPath(tmpDir, "dir2")
	resultDir := filepath.Join(dstDir, "dir1")
	dstPath := filepath.Join(resultDir, "file1-1")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)

	// Now try again but using a trailing path separator for dstDir.

	// unable to remove dstDir
	assert.Assert(c, os.RemoveAll(dstDir) == nil)

	// unable to make dstDir
	assert.Assert(c, os.MkdirAll(dstDir, os.FileMode(0755)) == nil)

	dstDir = cpPathTrailingSep(tmpDir, "dir2")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)
}

// H. SRC specifies a directory's contents only and DST does not exist. This
//    should create a directory at DST and copy the contents of the SRC
//    directory (but not the directory itself) into the DST directory. Ensure
//    this works whether DST has a trailing path separator or not.
func (s *DockerSuite) TestCpFromCaseH(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{addContent: true})

	tmpDir := getTestDir(c, "test-cp-from-case-h")
	defer os.RemoveAll(tmpDir)

	srcDir := containerCpPathTrailingSep(containerID, "dir1") + "."
	dstDir := cpPath(tmpDir, "testDir")
	dstPath := filepath.Join(dstDir, "file1-1")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)

	// Now try again but using a trailing path separator for dstDir.

	// unable to remove resultDir
	assert.Assert(c, os.RemoveAll(dstDir) == nil)

	dstDir = cpPathTrailingSep(tmpDir, "testDir")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)
}

// I. SRC specifies a directory's contents only and DST exists as a file. This
//    should cause an error as it is not possible to overwrite a file with a
//    directory.
func (s *DockerSuite) TestCpFromCaseI(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-i")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := containerCpPathTrailingSep(containerID, "/root/dir1") + "."
	dstFile := cpPath(tmpDir, "file1")

	err := runDockerCp(c, srcDir, dstFile, nil)
	assert.ErrorContains(c, err, "")

	assert.Assert(c, isCpCannotCopyDir(err), fmt.Sprintf("expected ErrCannotCopyDir error, but got %T: %s", err, err))
}

// J. SRC specifies a directory's contents only and DST exists as a directory.
//    This should copy the contents of the SRC directory (but not the directory
//    itself) into the DST directory. Ensure this works whether DST has a
//    trailing path separator or not.
func (s *DockerSuite) TestCpFromCaseJ(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	containerID := makeTestContainer(c, testContainerOptions{
		addContent: true, workDir: "/root",
	})

	tmpDir := getTestDir(c, "test-cp-from-case-j")
	defer os.RemoveAll(tmpDir)

	makeTestContentInDir(c, tmpDir)

	srcDir := containerCpPathTrailingSep(containerID, "/root/dir1") + "."
	dstDir := cpPath(tmpDir, "dir2")
	dstPath := filepath.Join(dstDir, "file1-1")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)

	// Now try again but using a trailing path separator for dstDir.

	// unable to remove dstDir
	assert.Assert(c, os.RemoveAll(dstDir) == nil)

	// unable to make dstDir
	assert.Assert(c, os.MkdirAll(dstDir, os.FileMode(0755)) == nil)

	dstDir = cpPathTrailingSep(tmpDir, "dir2")

	assert.Assert(c, runDockerCp(c, srcDir, dstDir, nil) == nil)

	assert.Assert(c, fileContentEquals(c, dstPath, "file1-1\n") == nil)
}
