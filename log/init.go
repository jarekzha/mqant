/**
 * description: 初始化
 * author: jarekzha@gmail.com
 * date: Aug 24, 2022
 */
package log

import (
	"fmt"

	"go.uber.org/zap"
)

var logger *zap.Logger
var sugaredLogger *zap.SugaredLogger

// 初始化
func Init(opts ...Option) {
	options := NewOptions(opts...)

	commonFields := zap.Fields(zap.String("process", options.ProcessID))

	var e error
	if options.Debug {
		logger, e = zap.NewDevelopment(commonFields)
	} else {
		logger, e = zap.NewProduction(commonFields)
	}

	sugaredLogger = logger.Sugar()
	zap.ReplaceGlobals(logger)

	if e != nil {
		panic(fmt.Sprintf("Log init fail: %v", e))
	}
}
