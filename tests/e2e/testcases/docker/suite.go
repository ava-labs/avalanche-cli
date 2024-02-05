// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apm

import (
	"fmt"
	"os/exec"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("[Docker]", func() {
	ginkgo.It("can build docker image for release", func() {
		cmd := exec.Command("make", "docker-e2e-build")
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		gomega.Expect(err).Should(gomega.BeNil())
	})
})
