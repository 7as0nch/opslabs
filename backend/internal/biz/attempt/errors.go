/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 领域错误
**/
package attempt

import "errors"

// ErrAttemptNotFound data 层找不到记录时返回,biz 层会翻译成 Kratos errors
var ErrAttemptNotFound = errors.New("attempt not found")
