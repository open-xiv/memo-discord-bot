package model

import (
	"time"

	"github.com/lib/pq"
)

type Privacy int

const (
	PrivacyPublic   Privacy = 0
	PrivacyUnranked Privacy = 1
	PrivacyHidden   Privacy = 2
)

type Member struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `gorm:"uniqueIndex:idx_member_name_server" json:"name"`
	Server string `gorm:"uniqueIndex:idx_member_name_server" json:"server"`

	Privacy      Privacy        `gorm:"column:privacy;type:smallint;not null;default:0" json:"-"`
	Tags         pq.StringArray `gorm:"type:text[];index:,type:gin" json:"tags"`
	LogsSyncTime *time.Time     `json:"logs_sync_time"`
}
