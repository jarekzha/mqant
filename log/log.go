/**
 * description: log
 * author: jarekzha@gmail.com
 * date: Aug 25, 2022
 */
package log

import "go.uber.org/zap"

// Debug Debug
func Debug(format string, a ...interface{}) {
	//gLogger.doPrintf(debugLevel, printDebugLevel, format, a...)
	zap.S().Debugf(format, a...)
}

// Info Info
func Info(format string, a ...interface{}) {
	//gLogger.doPrintf(releaseLevel, printReleaseLevel, format, a...)
	zap.S().Infof(format, a...)
}

// Error Error
func Error(format string, a ...interface{}) {
	//gLogger.doPrintf(errorLevel, printErrorLevel, format, a...)
	zap.S().Errorf(format, a...)
}

// Warning Warning
func Warning(format string, a ...interface{}) {
	//gLogger.doPrintf(fatalLevel, printFatalLevel, format, a...)
	zap.S().Warnf(format, a...)
}
