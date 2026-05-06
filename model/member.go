package model

import (
	"time"

	"github.com/lib/pq"
)

type Member struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `gorm:"uniqueIndex:idx_member_name_server" json:"name"`
	Server string `gorm:"uniqueIndex:idx_member_name_server" json:"server"`

	Hidden       bool           `gorm:"default:false" json:"hidden"`
	Tags         pq.StringArray `gorm:"type:text[];index:,type:gin" json:"tags"`
	LogsSyncTime *time.Time     `json:"logs_sync_time"`
}
