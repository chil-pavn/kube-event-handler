package controllers

import (
	"os"
	"strconv"

	ctrl "sigs.k8s.io/controller-runtime"
)

var debugLogger = ctrl.Log.WithName("debug")

// IsDebugMode returns true if debug mode is enabled
func IsDebugMode() bool {
    debug, _ := strconv.ParseBool(os.Getenv("DEBUG_MODE"))
    return debug
}

// DebugLog logs a message if debug mode is enabled
func DebugLog(msg string, keysAndValues ...interface{}) {
    if IsDebugMode() {
        debugLogger.Info(msg, keysAndValues...)
    }
} 