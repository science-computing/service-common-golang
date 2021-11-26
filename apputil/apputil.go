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
	logger = InitLogging()

	logger.Info("Init configuration")
	if explicitConfigFilename == "" {
		explicitConfigFilename = os.Getenv(fmt.Sprintf("%s_%s_CONFIG", strings.ToUpper(projectName), strings.ToUpper(serviceName)))
	}
	viper.SetConfigType("yaml")
	if explicitConfigFilename != "" {
		f, err := os.Open(explicitConfigFilename)
		defer f.Close()
		if err != nil {
			logger.Fatalf("Configfile %s could not be read: %v", explicitConfigFilename, err)
		}
		err = viper.ReadConfig(f)
		if err != nil {
			logger.Fatalf("Configfile %s could not be read: %v", explicitConfigFilename, err)
		}
		logger.Infof("Successfully read configuration from [%v]", explicitConfigFilename)
	} else {
		configdir := os.Getenv(fmt.Sprintf("%s_CONFIGDIR", strings.ToUpper(projectName)))
		if configdir == "" {
			logger.Fatalf("No Configuration specified")
		}
		viper.AddConfigPath(configdir)
		// convenient for development to use workspace local config file
		viper.SetConfigName(serviceName)
		err := viper.ReadInConfig()
		if err != nil {
			logger.Fatalf("Configuration could not be read: %s", err)
		}
		logger.Infof("Successfully read configuration from [%v]", viper.GetViper().ConfigFileUsed())
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

//InitLoggingWithLevel inits apex/log as log
func InitLoggingWithLevel(level log.Level) *log.Entry {
	if logger == nil {
		// init logging
		log.SetHandler(verbosetextlog.New(os.Stdout))
		logger = log.WithFields(log.Fields{})

		// set default log level to INFO
		log.SetLevel(level)
	}

	return logger
}

//InitLogging inits apex/log as log handler and set default level to INFO
func InitLogging() *log.Entry {
	return InitLoggingWithLevel(log.InfoLevel)
}

// GenerateGUID generates a globally unique identifier
func GenerateGUID() string {
	return uuid.New().String()
}
