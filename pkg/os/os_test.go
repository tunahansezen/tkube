package os

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"testing"
)

func TestOS_Version(t *testing.T) {
	versionStr := "26.1.3-1.el7"
	semVer, err := version.NewSemver(versionStr)
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Printf("sem ver: %s\n", semVer.String())
}
