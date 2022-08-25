/**
 * description: log
 * author: jarekzha@gmail.com
 * date: Aug 25, 2022
 */
package log

import "go.uber.org/zap"

// Warning Warning
func Warning(format string, a ...interface{}) {
	//gLogger.doPrintf(fatalLevel, printFatalLevel, format, a...)
	zap.S().Warnf(format, a...)
}
