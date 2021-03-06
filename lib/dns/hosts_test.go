package dns

import (
	"testing"

	"github.com/shuLhan/share/lib/test"
)

func TestHostsLoad(t *testing.T) {
	msgs, err := HostsLoad("testdata/hosts")
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, "Length", 10, len(msgs), true)
}

func TestHostsLoad2(t *testing.T) {
	_, err := HostsLoad("testdata/hosts.block")
	if err != nil {
		t.Fatal(err)
	}
}
