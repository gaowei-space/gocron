package models

import "time"

const (
	AgentAuthStatusPending  = "pending"
	AgentAuthStatusApproved = "approved"
	AgentAuthStatusDenied   = "denied"
	AgentAuthStatusExpired  = "expired"
)

type AgentDeviceAuthorization struct {
	Id                int       `json:"id" xorm:"pk autoincr notnull"`
	UserId            int       `json:"user_id" xorm:"int notnull index"`
	DeviceId          string    `json:"device_id" xorm:"varchar(64) notnull unique"`
	DeviceName        string    `json:"device_name" xorm:"varchar(128) notnull default ''"`
	ClientType        string    `json:"client_type" xorm:"varchar(32) notnull default ''"`
	ClientVersion     string    `json:"client_version" xorm:"varchar(32) notnull default ''"`
	RefreshTokenHash  string    `json:"-" xorm:"char(64) notnull default ''"`
	PreviousTokenHash string    `json:"-" xorm:"char(64) notnull default ''"`
	ExpiresAt         time.Time `json:"expires_at" xorm:"datetime notnull"`
	RevokedAt         time.Time `json:"revoked_at" xorm:"datetime"`
	LastUsedAt        time.Time `json:"last_used_at" xorm:"datetime"`
	LastUsedIp        string    `json:"last_used_ip" xorm:"varchar(64) notnull default ''"`
	CreatedAt         time.Time `json:"created_at" xorm:"datetime notnull created"`
	UpdatedAt         time.Time `json:"updated_at" xorm:"datetime updated"`
	BaseModel         `json:"-" xorm:"-"`
}

func (a *AgentDeviceAuthorization) Create() (int, error) {
	_, err := Db.Insert(a)
	if err != nil {
		return 0, err
	}
	return a.Id, nil
}

func (a *AgentDeviceAuthorization) FindByDeviceId(deviceId string) (bool, error) {
	return Db.Where("device_id = ?", deviceId).Get(a)
}

func (a *AgentDeviceAuthorization) FindByRefreshTokenHash(hash string) (bool, error) {
	return Db.Where("refresh_token_hash = ?", hash).Get(a)
}

func (a *AgentDeviceAuthorization) FindByPreviousTokenHash(hash string) (bool, error) {
	return Db.Where("previous_token_hash = ?", hash).Get(a)
}

func (a *AgentDeviceAuthorization) UpdateById(id int, data CommonMap) (int64, error) {
	return Db.Table(a).ID(id).Update(data)
}

func (a *AgentDeviceAuthorization) List(params CommonMap) ([]AgentDeviceAuthorization, error) {
	a.parsePageAndPageSize(params)
	list := make([]AgentDeviceAuthorization, 0)
	err := Db.Desc("id").Limit(a.PageSize, a.pageLimitOffset()).Find(&list)
	return list, err
}

type AgentDeviceAuthorizationRequest struct {
	Id             int       `json:"id" xorm:"pk autoincr notnull"`
	DeviceCodeHash string    `json:"-" xorm:"char(64) notnull unique"`
	UserCodeHash   string    `json:"-" xorm:"char(64) notnull unique"`
	DeviceName     string    `json:"device_name" xorm:"varchar(128) notnull default ''"`
	ClientType     string    `json:"client_type" xorm:"varchar(32) notnull default ''"`
	ClientVersion  string    `json:"client_version" xorm:"varchar(32) notnull default ''"`
	Status         string    `json:"status" xorm:"varchar(16) notnull index default 'pending'"`
	ApprovedBy     int       `json:"approved_by" xorm:"int notnull default 0"`
	LastPolledAt   time.Time `json:"last_polled_at" xorm:"datetime"`
	PollCount      int       `json:"poll_count" xorm:"int notnull default 0"`
	ExpiresAt      time.Time `json:"expires_at" xorm:"datetime notnull"`
	CreatedAt      time.Time `json:"created_at" xorm:"datetime notnull created"`
	UpdatedAt      time.Time `json:"updated_at" xorm:"datetime updated"`
}

func (a *AgentDeviceAuthorizationRequest) Create() (int, error) {
	_, err := Db.Insert(a)
	if err != nil {
		return 0, err
	}
	return a.Id, nil
}

func (a *AgentDeviceAuthorizationRequest) FindByDeviceCodeHash(hash string) (bool, error) {
	return Db.Where("device_code_hash = ?", hash).Get(a)
}

func (a *AgentDeviceAuthorizationRequest) FindByUserCodeHash(hash string) (bool, error) {
	return Db.Where("user_code_hash = ?", hash).Get(a)
}

func (a *AgentDeviceAuthorizationRequest) UpdateById(id int, data CommonMap) (int64, error) {
	return Db.Table(a).ID(id).Update(data)
}

func (a *AgentDeviceAuthorizationRequest) ClaimApproved(id int) (bool, error) {
	affected, err := Db.Table(a).
		Where("id = ? AND status = ?", id, AgentAuthStatusApproved).
		Update(CommonMap{"status": AgentAuthStatusExpired})
	return affected > 0, err
}

type AgentAuditLog struct {
	Id             int       `json:"id" xorm:"pk autoincr notnull"`
	RequestId      string    `json:"request_id" xorm:"varchar(64) notnull index default ''"`
	UserId         int       `json:"user_id" xorm:"int notnull index default 0"`
	DeviceId       string    `json:"device_id" xorm:"varchar(64) notnull index default ''"`
	ClientType     string    `json:"client_type" xorm:"varchar(32) notnull default ''"`
	ClientVersion  string    `json:"client_version" xorm:"varchar(32) notnull default ''"`
	SourceIp       string    `json:"source_ip" xorm:"varchar(64) notnull default ''"`
	Source         string    `json:"source" xorm:"varchar(32) notnull default ''"`
	Action         string    `json:"action" xorm:"varchar(64) notnull index default ''"`
	TargetType     string    `json:"target_type" xorm:"varchar(32) notnull default ''"`
	TargetId       string    `json:"target_id" xorm:"varchar(64) notnull default ''"`
	RequestSummary string    `json:"request_summary" xorm:"varchar(512) notnull default ''"`
	Success        bool      `json:"success" xorm:"bool notnull default 0"`
	ErrorMessage   string    `json:"error_message" xorm:"varchar(512) notnull default ''"`
	CreatedAt      time.Time `json:"created_at" xorm:"datetime notnull created"`
}

func (a *AgentAuditLog) Create() (int, error) {
	_, err := Db.Insert(a)
	if err != nil {
		return 0, err
	}
	return a.Id, nil
}
