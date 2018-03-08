package varroa

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testQuotaOutput = `Disk quotas for user username (uid 1234):
     Filesystem  blocks   quota   limit   grace   files   quota   limit   grace
     /dev/md127 265117776  314572800       0           92837       0       0        `
)

func TestQuota(t *testing.T) {
	fmt.Println("\n --- Testing Quota parsing. ---")
	check := assert.New(t)

	ratio, remaining, err := parseQuota(testQuotaOutput)
	check.Nil(err)
	check.Equal(float32(84.27867), ratio)
	check.Equal(int64(1024*49455024), remaining)
}

/*
func TestFreeDiskSpace(t *testing.T) {
	fmt.Println("\n --- Testing Free disk space ---")
	check := assert.New(t)

	// setup logger
	c := &Config{General: &ConfigGeneral{DownloadDir: "test", LogLevel: 2}, DownloadFolderConfigured: true}
	env := &Environment{config: c}
	logThis = NewLogThis(env)

	check.Nil(checkFreeDiskSpace())
}
*/
