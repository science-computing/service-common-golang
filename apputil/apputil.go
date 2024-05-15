// Package apputil provides basic application functions
package apputil

import (
	"fmt"
	"os"
	"strings"

	"github.com/science-computing/service-common-golang/apputil/verbosetextlog"

	"github.com/apex/log"
	"github.com/google/uuid"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const debugLogLevelConfigKey = "debug"

var (
	logger                 *log.Entry
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
		log.SetLevel(log.DebugLevel)
	}

	// print config
	for key, value := range viper.AllSettings() {
		logger.Debugf("Configuration setting [%s=%v]", key, value)
	}
}

// InitLogging inits apex/log as log
func InitLoggingWithLevel(level log.Level) *log.Entry {
	if logger == nil {
		logger = log.WithFields(log.Fields{})
	}
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
	// init logging
	log.SetHandler(verbosetextlog.New(logfile))

	// set default log level to INFO
	log.SetLevel(level)

	return logger
}

// InitLogging inits apex/log as log handler and set default level to INFO
func InitLogging() *log.Entry {
	return InitLoggingWithLevel(log.InfoLevel)
}

// GenerateGUID generates a globally unique identifier
func GenerateGUID() string {
	return uuid.New().String()
}
