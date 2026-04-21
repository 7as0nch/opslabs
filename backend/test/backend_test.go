package test

/* *
 * @Author: chengjiang
 * @Date: 2025-12-16 17:08:52
 * @Description:
**/

import (
	"testing"

	"github.com/7as0nch/backend/tools"
)

// test snowflake id
func TestSnowflakeID(t *testing.T) {
	id := tools.GetSFID()
	t.Log("snowflake id:", id)
}
