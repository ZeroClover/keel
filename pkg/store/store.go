package store

import (
	"errors"

	"github.com/keel-hq/keel/types"
)

type Store interface {
	CreateAuditLog(entry *types.AuditLog) (id string, err error)
	GetAuditLogs(query *types.AuditLogQuery) (logs []*types.AuditLog, err error)
	AuditLogsCount(query *types.AuditLogQuery) (int, error)
	AuditStatistics(query *types.AuditLogStatsQuery) ([]types.AuditLogStats, error)

	OK() bool
	Close() error
}

// errors
var (
	ErrRecordNotFound = errors.New("record not found")
)
