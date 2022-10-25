/**
 * description: 字段
 * author: jarekzha@gmail.com
 * date: Oct 25, 2022
 */

package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Sync() {
	zap.L().Sync()
	zap.S().Sync()
}

func Log(lvl zapcore.Level, msg string, fields ...zap.Field) {
	zap.L().Log(lvl, msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	zap.L().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	zap.L().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	zap.L().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	zap.L().Error(msg, fields...)
}

func DPanic(msg string, fields ...zap.Field) {
	zap.L().DPanic(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	zap.L().Panic(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	zap.L().Fatal(msg, fields...)
}

func Debugf(template string, args ...interface{}) {
	zap.S().Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	zap.S().Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	zap.S().Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	zap.S().Errorf(template, args...)
}

func DPanicf(template string, args ...interface{}) {
	zap.S().DPanicf(template, args...)
}

func Panicf(template string, args ...interface{}) {
	zap.S().Panicf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	zap.S().Fatalf(template, args...)
}
