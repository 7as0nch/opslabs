// @author <chengjiang@buffalo-robot.com>
// @date 2023/2/16
// @note
package tools

import (
	"fmt"
	"github.com/gofrs/uuid"
	"strings"
	"testing"
)

func TestUUID(t *testing.T) {
	fmt.Println(uuid.NewV1())
	fmt.Println(strings.ReplaceAll(uuid.Must(uuid.NewV4()).String(), "-", ""))

	fmt.Println(GetSnowID(), GetSFID())
}

func TestPwd(t *testing.T) {
	println(Encode("123456"))
	println(Check("123456", Encode("123456")))
	fmt.Println(RandStringBytesMaskImprSrcUnsafe(5))
}
