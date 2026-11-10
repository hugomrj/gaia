package tenant

import "time"

type Tenant struct {
	ID           int64
	Slug         string
	Name         string
	Provider     string
	APIKeyPlain  string // dev
	APIKeyEnc    []byte // prod
	TokenHash    string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (t *Tenant) IsActive() bool {
	return t.Status == "active"
}