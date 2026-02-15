package model

import (
	"github.com/lib/pq"
)

type User struct {
	ID        uint    `gorm:"primaryKey"`
	DiscordID *string `gorm:"uniqueIndex"`

	RoleIDs pq.StringArray `gorm:"type:text[];column:role_ids;index:,type:gin"`

	Members []Member `gorm:"many2many:user_members;"`
}
