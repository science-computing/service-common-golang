// Package apputil provides basic application functions
package apputil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/science-computing/service-common-golang/apputil/slogverbosetext"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const debugLogLevelConfigKey = "debug"

// LoggerWrapper wraps slog.Logger to provide apex/log compatible interface
type LoggerWrapper struct {
	*slog.Logger
}

// Debug provides apex/log compatible debug logging (without format)
func (l *LoggerWrapper) Debug(message string) {
	l.log(slog.LevelDebug, message)
}

// Debugf provides apex/log compatible debug logging
func (l *LoggerWrapper) Debugf(format string, args ...interface{}) {
	l.log(slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Info provides apex/log compatible info logging (without format)
func (l *LoggerWrapper) Info(message string) {
	l.log(slog.LevelInfo, message)
}

// Infof provides apex/log compatible info logging
func (l *LoggerWrapper) Infof(format string, args ...interface{}) {
	l.log(slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Warn provides apex/log compatible warn logging (without format)
func (l *LoggerWrapper) Warn(message string) {
	l.log(slog.LevelWarn, message)
}

// Warnf provides apex/log compatible warn logging
func (l *LoggerWrapper) Warnf(format string, args ...interface{}) {
	l.log(slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Error provides apex/log compatible error logging (without format)
func (l *LoggerWrapper) Error(message string) {
	l.log(slog.LevelError, message)
}

// Errorf provides apex/log compatible error logging
func (l *LoggerWrapper) Errorf(format string, args ...interface{}) {
	l.log(slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatal provides apex/log compatible fatal logging (without format)
func (l *LoggerWrapper) Fatal(message string) {
	l.log(slog.LevelError, message)
	os.Exit(1)
}

// Fatalf provides apex/log compatible fatal logging
func (l *LoggerWrapper) Fatalf(format string, args ...interface{}) {
	l.log(slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// log is a helper that logs with the correct source location
// It dynamically finds the first caller outside of apputil.go
func (l *LoggerWrapper) log(level slog.Level, message string) {
	ctx := context.Background()
	if !l.Logger.Enabled(ctx, level) {
		return
	}

	// Get multiple program counters to find the first one outside apputil.go
	var pcs [10]uintptr
	n := runtime.Callers(2, pcs[:]) // Start from 2 to skip runtime.Callers and this function

	// Find the first PC that's not in apputil.go by checking each frame
	var pc uintptr
	for i := range n {
		frames := runtime.CallersFrames(pcs[i : i+1])
		frame, _ := frames.Next()
		// Skip frames from apputil.go (our wrapper methods)
		if !strings.Contains(frame.File, "apputil.go") {
			pc = pcs[i]
			break
		}
	}

	// Fallback: use the first frame if we couldn't find one outside apputil.go
	if pc == 0 {
		pc = pcs[0]
	}

	r := slog.NewRecord(time.Now(), level, message, pc)
	_ = l.Logger.Handler().Handle(ctx, r)
}

var (
	logger                 *LoggerWrapper
	explicitConfigFilename string
	upperProjectName       string
	upperServiceName       string
)

func init() {
	pflag.StringVar(&explicitConfigFilename, "config", "", "the configfile to use")
}

// SetExplicitConfigFile overrides the heuristics to identify the config file
func SetExplicitConfigFile(name string) {
	explicitConfigFilename = name
}

// InitConfig inits Viper configuration, i.e. setting config search path to /etc/config/$SERVICE_NAME
// requiredKeys are checked for presence to ensure default configuration values
// if not found the service exits
func InitConfig(projectName string, serviceName string, requiredKeys []string) {
	upperProjectName = strings.ReplaceAll(strings.ToUpper(projectName), "-", "_")
	upperServiceName = strings.ReplaceAll(strings.ToUpper(serviceName), "-", "_")
	if logger == nil {
		logger = InitLogging()
	}
	logger.Debug("Init configuration")
	if explicitConfigFilename == "" {
		explicitConfigFilename = os.Getenv(fmt.Sprintf("%s_%s_CONFIG", upperProjectName, upperServiceName))
	}
	viper.SetConfigType("yaml")
	if explicitConfigFilename != "" {
		f, err := os.Open(explicitConfigFilename)
		if err != nil {
			logger.Fatalf("Configfile %s could not be read: %v", explicitConfigFilename, err)
		}
		defer f.Close()
		err = viper.ReadConfig(f)
		if err != nil {
			logger.Fatalf("Configfile %s could not be read: %v", explicitConfigFilename, err)
		}
		logger.Infof("Successfully read configuration from [%v]", explicitConfigFilename)
	} else {
		configDirName := fmt.Sprintf("%s_CONFIGDIR", upperProjectName)
		configDir := os.Getenv(configDirName)
		// if CAEF_CONFIGDIR is not specified, look for config in /etc
		if configDir == "" {
			configDir = "/etc"
			logger.Debugf("%s not specified in env. Looking for /etc/%s", configDirName, strings.ToLower(serviceName)+".yaml")
		}
		viper.AddConfigPath(configDir)
		viper.SetConfigName(serviceName)
		err := viper.ReadInConfig()
		if err != nil {
			logger.Fatalf("Configuration could not be read: %s", err)
		}
		logger.Debugf("Successfully read configuration from [%v]", viper.GetViper().ConfigFileUsed())
	}

	// overwrite config file config values with ENV values if present
	viper.SetEnvPrefix(fmt.Sprintf("%s_%s", strings.ToUpper(projectName), strings.ToUpper(serviceName)))
	// tells viper to check for the env var everytime Get() is called
	// the name is assumed to be DATASET_MYVAR
	viper.AutomaticEnv()

	// check values
	for _, key := range requiredKeys {
		if !viper.IsSet(key) {
			logger.Fatalf("No config key [%s] in config file or [%s] in ENV", key, strings.ToUpper(serviceName)+"_"+strings.ToUpper(key))
		}
	}

	// check if debug log is enabled via config file or ENV
	if viper.GetBool(debugLogLevelConfigKey) {
		//set Viper internal log level to output everything
		jww.SetLogThreshold(jww.LevelTrace)
		jww.SetStdoutThreshold(jww.LevelTrace)
		// Reinitialize logger with debug level
		logger.Logger = createSLogger(slog.LevelDebug)
	}
}

// createSLogger creates a new logger with the specified level
// This is an internal function used to recreate the logger when the level needs to change
func createSLogger(level slog.Level) *slog.Logger {
	logfilename := ""
	if upperProjectName != "" && upperServiceName != "" {
		logfilename = os.Getenv(fmt.Sprintf("%s_%s_LOGFILE", upperProjectName, upperServiceName))
	}
	if logfilename == "" {
		logfilename = "stdout"
	}
	var logfile *os.File
	if logfilename == "stdout" {
		logfile = os.Stdout
	} else if logfilename == "stderr" {
		logfile = os.Stderr
	} else {
		logfile, _ = os.OpenFile(logfilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	}

	// Create our custom verbose text handler that matches the original format
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	handler := slogverbosetext.New(logfile, opts)
	slogger := slog.New(handler)
	return slogger
}

// InitLoggingWithLevel inits slog with specified level and returns a LoggerWrapper
// Uses singleton pattern - only creates logger once
func InitLoggingWithLevel(level slog.Level) *LoggerWrapper {
	slogger := createSLogger(level)
	if logger == nil {
		logger = &LoggerWrapper{Logger: slogger}
	} else {
		logger.Logger = slogger
	}
	return logger
}

// InitLogging inits slog as log handler and set default level to INFO
// Uses singleton pattern - only creates logger once
func InitLogging() *LoggerWrapper {
	if logger == nil {
		logger = &LoggerWrapper{Logger: createSLogger(slog.LevelInfo)}
	}
	return logger
}

// GenerateGUID generates a globally unique identifier
func GenerateGUID() string {
	return uuid.New().String()
}
