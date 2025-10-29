// Package apputil provides basic application functions
package apputil

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

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
	l.Logger.Debug(message)
}

// Debugf provides apex/log compatible debug logging
func (l *LoggerWrapper) Debugf(format string, args ...interface{}) {
	l.Logger.Debug(fmt.Sprintf(format, args...))
}

// Info provides apex/log compatible info logging (without format)
func (l *LoggerWrapper) Info(message string) {
	l.Logger.Info(message)
}

// Infof provides apex/log compatible info logging
func (l *LoggerWrapper) Infof(format string, args ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, args...))
}

// Warn provides apex/log compatible warn logging (without format)
func (l *LoggerWrapper) Warn(message string) {
	l.Logger.Warn(message)
}

// Warnf provides apex/log compatible warn logging
func (l *LoggerWrapper) Warnf(format string, args ...interface{}) {
	l.Logger.Warn(fmt.Sprintf(format, args...))
}

// Error provides apex/log compatible error logging (without format)
func (l *LoggerWrapper) Error(message string) {
	l.Logger.Error(message)
}

// Errorf provides apex/log compatible error logging
func (l *LoggerWrapper) Errorf(format string, args ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, args...))
}

// Fatal provides apex/log compatible fatal logging (without format)
func (l *LoggerWrapper) Fatal(message string) {
	l.Logger.Error(message)
	os.Exit(1)
}

// Fatalf provides apex/log compatible fatal logging
func (l *LoggerWrapper) Fatalf(format string, args ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
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
		logger = InitLoggingWithLevel(slog.LevelDebug)
	}
}

// InitLoggingWithLevel inits slog with specified level and returns a LoggerWrapper
func InitLoggingWithLevel(level slog.Level) *LoggerWrapper {
	if logger == nil {
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
		logger = &LoggerWrapper{Logger: slogger}
	}
	return logger
}

// InitLogging inits slog as log handler and set default level to INFO
func InitLogging() *LoggerWrapper {
	return InitLoggingWithLevel(slog.LevelInfo)
}

// GenerateGUID generates a globally unique identifier
func GenerateGUID() string {
	return uuid.New().String()
}
