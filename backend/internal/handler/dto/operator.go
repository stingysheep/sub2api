package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

// Operator DTOs expose only the dedicated read models, never the admin entities.
type OperatorUser = service.OperatorUser
type OperatorGroup = service.OperatorGroup
type OperatorOrder = service.OperatorOrder
type OperatorUsage = service.OperatorUsage
type OperatorBalanceHistory = service.OperatorBalanceHistory
type OperatorChannelStatus = service.OperatorChannelStatus
