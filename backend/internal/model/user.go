package model

import "time"

type User struct {
	ID           string    `json:"id" bun:",pk,type:varchar(36)"`
	Email        string    `json:"email" bun:",unique,type:varchar(255),notnull"`
	Name         string    `json:"name" bun:",type:varchar(255),notnull"`
	PasswordHash string    `json:"-" bun:",type:varchar(255),notnull"`
	CreatedAt    time.Time `json:"created_at" bun:",type:timestamptz,notnull,default:now()"`
	UpdatedAt    time.Time `json:"updated_at" bun:",type:timestamptz,notnull,default:now()"`
}

func (User) TableName() string {
	return "users"
}

type RefreshToken struct {
	ID          string     `json:"id" bun:",pk,type:varchar(36)"`
	UserID      string     `json:"user_id" bun:",type:varchar(36),notnull"`
	TokenHash   string     `json:"-" bun:",type:varchar(64),notnull"`
	ReplacedBy  *string    `json:"replaced_by,omitempty" bun:",type:varchar(36)"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty" bun:",type:timestamptz"`
	ExpiresAt   time.Time  `json:"expires_at" bun:",type:timestamptz,notnull"`
	CreatedAt   time.Time  `json:"created_at" bun:",type:timestamptz,notnull,default:now()"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
