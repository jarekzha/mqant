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

	zapOptions := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
	}

	var e error
	if options.Debug {
		logger, e = zap.NewDevelopment(zapOptions...)
	} else {
		zapOptions = append(zapOptions, zap.Fields(zap.String("process", options.ProcessID)))
		logger, e = zap.NewProduction(zapOptions...)
	}

	sugaredLogger = logger.Sugar()
	zap.ReplaceGlobals(logger)

	if e != nil {
		panic(fmt.Sprintf("Log init fail: %v", e))
	}
}
