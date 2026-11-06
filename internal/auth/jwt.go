package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload for platform-hosted game servers.
// Subject identifies the game server / client; MSPID and Role carry the
// Fabric identity attributes the gateway uses for chaincode authorization.
type Claims struct {
	MSPID string `json:"mspId"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// Manager issues and validates JWTs for platform-hosted clients.
type Manager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

// NewManager returns a JWT manager. secret must be at least 32 bytes in
// production; callers should fail fast if a weaker secret is supplied.
func NewManager(secret, issuer, audience string, ttl time.Duration) (*Manager, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &Manager{secret: []byte(secret), issuer: issuer, audience: audience, ttl: ttl, now: time.Now}, nil
}

// Issue creates a signed JWT for the given subject, MSP and role.
func (m *Manager) Issue(subject, mspID, role string) (string, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}
	now := m.now()
	claims := Claims{
		MSPID: mspID,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Validate parses and verifies a token, returning its claims.
func (m *Manager) Validate(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
