package auditutil

import (
	"github.com/apex/log"
	"github.com/science-computing/service-common-golang/auditutil/auditlog"
)

var (
	logger *log.Entry
)

//InitAuditLoggingWithLevel inits apex/log as audit log
func InitAuditLoggingWithLevel(level log.Level) *log.Entry {
	if logger == nil {
		// init logging
		logger = auditlog.WithFields(log.Fields{})
		auditlog.SetLevel(level)
	}

	return logger
}

//InitAuditLogging inits apex/log as audit log handler and set default level to INFO
func InitAuditLogging() *log.Entry {
	return InitAuditLoggingWithLevel(log.InfoLevel)
}
