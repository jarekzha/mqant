/**
 * description: 字段
 * author: jarekzha@gmail.com
 * date: Aug 24, 2022
 */
package log

import (
	"fmt"

	"go.uber.org/zap"
)

func Span(span TraceSpan) zap.Field {
	str := fmt.Sprintf("%s.%s", span.TraceID(), span.SpanID())
	return zap.String("span", str)
}
