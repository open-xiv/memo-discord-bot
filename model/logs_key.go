package model

import "time"

type LogsKey struct {
	ID uint `gorm:"primaryKey"`

	UserID uint  `gorm:"uniqueIndex"`
	User   *User `gorm:"foreignKey:UserID"`

	Client string `gorm:"uniqueIndex"`
	Secret string

	UpdatedAt time.Time
	LastUseAt time.Time

	UseCount uint
	ErrCount uint
}
