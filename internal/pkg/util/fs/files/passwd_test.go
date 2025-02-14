// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sylabs/singularity/internal/pkg/test"
	"gotest.tools/v3/golden"
)

func TestPasswd(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	uid := os.Getuid()

	// Test how Passwd() works with a bad passwd file
	_, err := Passwd("/fake", "/fake", uid, nil)
	if err == nil {
		t.Errorf("should have failed with bad passwd file")
	}

	// Test how Passwd() works with an empty file
	f, err := os.CreateTemp("", "empty-passwd-")
	if err != nil {
		t.Error(err)
	}
	emptyPasswd := f.Name()
	defer os.Remove(emptyPasswd)
	f.Close()

	_, err = Passwd(emptyPasswd, "/home", uid, nil)
	if err != nil {
		t.Error(err)
	}

	inputPasswdFilePath := filepath.Join(".", "testdata", "passwd.in")
	testUID := 0
	testHomeDir := "/tmp"
	testGoldenFile := "passwd.root.customhome.golden"
	bytes, err := Passwd(inputPasswdFilePath, testHomeDir, testUID, nil)
	if err != nil {
		t.Errorf("Unexpected error encountered calling Passwd(): %v", err)
		return
	}

	golden.Assert(t, string(bytes), testGoldenFile, "mismatch in Passwd() invocation (uid: %d; requested homeDir: %#v)", testUID, testHomeDir)
}
