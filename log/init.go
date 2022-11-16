/**
 * description: 初始化
 * author: jarekzha@gmail.com
 * date: Aug 24, 2022
 */
package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 初始化
func Init(opts ...Option) {
	options := NewOptions(opts...)

	zapOptions := []zap.Option{zap.AddCallerSkip(1)}

	var config zap.Config
	if options.Debug {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		// caller预留长度，用于对齐后续msg
		callerBlank := "                                    "
		config.EncoderConfig.EncodeCaller = func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			path := caller.TrimmedPath()
			pathLen := len(path)
			if pathLen < len(callerBlank) {
				enc.AppendString(path + callerBlank[pathLen:])
			} else {
				enc.AppendString(path)
			}
		}
	} else {
		config = zap.NewProductionConfig()
		zapOptions = append(zapOptions, zap.Fields(zap.String("process", options.ProcessID)))
	}

	logger, e := config.Build(zapOptions...)
	if e != nil {
		panic(fmt.Sprintf("Log init fail: %v", e))
	}

	// 替换全局
	zap.ReplaceGlobals(logger)
}
