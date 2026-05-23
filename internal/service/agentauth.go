package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/ouqiang/gocron/internal/modules/app"
	"github.com/ouqiang/gocron/internal/modules/utils"
)

const AgentRefreshTokenLength int64 = 48
const AgentAccessTokenDuration = 30 * time.Minute
const AgentDeviceAuthorizationDuration = 30 * 24 * time.Hour
const AgentDeviceCodeDuration = 10 * time.Minute
const AgentDeviceCodePollInterval = 5

var ErrAgentRefreshTokenInvalid = errors.New("invalid refresh token")

type AgentDeviceSession struct {
	RefreshTokenHash  string
	PreviousTokenHash string
}

func HashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewAgentRefreshToken() string {
	return utils.RandAuthToken() + utils.RandString(AgentRefreshTokenLength)
}

func NewAgentDeviceCode() string {
	return utils.RandAuthToken()
}

func NewAgentUserCode() string {
	return utils.RandString(8)
}

func NewAgentDeviceId() string {
	return utils.RandAuthToken()
}

func RotateAgentRefreshToken(device *AgentDeviceSession, presentedToken string) (string, error) {
	if device == nil || device.RefreshTokenHash == "" {
		return "", ErrAgentRefreshTokenInvalid
	}
	if HashAgentToken(presentedToken) != device.RefreshTokenHash {
		return "", ErrAgentRefreshTokenInvalid
	}

	nextToken := NewAgentRefreshToken()
	device.PreviousTokenHash = device.RefreshTokenHash
	device.RefreshTokenHash = HashAgentToken(nextToken)

	return nextToken, nil
}

func GenerateAgentAccessToken(uid int, deviceId, clientType, clientVersion string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	now := time.Now()
	claims := make(jwt.MapClaims)
	claims["exp"] = now.Add(AgentAccessTokenDuration).Unix()
	claims["iat"] = now.Unix()
	claims["issuer"] = "gocron-agent"
	claims["uid"] = uid
	claims["device_id"] = deviceId
	claims["client_type"] = clientType
	claims["client_version"] = clientVersion
	token.Claims = claims

	return token.SignedString([]byte(app.Setting.AuthSecret))
}

func ParseAgentAccessToken(rawToken string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(rawToken, func(*jwt.Token) (interface{}, error) {
		return []byte(app.Setting.AuthSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid access token")
	}

	return claims, nil
}
