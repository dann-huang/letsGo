package config

import (
	"fmt"
	"os"
	"time"
)

type Auth struct {
	AccCookieName string
	AccSecret     string
	AccTTL        time.Duration
	RefCookieName string
	RefreshKey    string
	RefTTL        time.Duration
	Issuer        string
	Audience      string
	EmailCodeTTL  time.Duration
	EmailSetupKey string
	EmailLoginKey string
	EmailPassKey  string
}

type DB struct {
	PostgresUrl  string
	PostgresUser string
	PostgresPass string
	PostgresDB   string
	RedisURL     string
}

type WS struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PongTimeout  time.Duration
	MaxMsgSize   int64

	RegisterBuffer int64
	RoomBuffer     int64
	MsgBuffer      int64
	SendBuffer     int64
	RecvBuffer     int64
}

type Mail struct {
	MailKey  string
	MailFrom string
}

type Token struct {
	RefTTL         time.Duration
	EmailedCodeTTL time.Duration
}

type AppConfig struct {
	// Port        string
	// FrontendUrl string
	StaticPages string
	Auth        *Auth
	DB          *DB
	WS          *WS
	Mail        *Mail
	Token       *Token
}

func Load() (*AppConfig, error) {
	cfg := &AppConfig{
		// Port:        os.Getenv("GO_PORT"),
		// FrontendUrl: os.Getenv("FRONTEND"),
		StaticPages: "/app/static",
		Mail: &Mail{
			MailKey:  os.Getenv("RESEND_KEY"),
			MailFrom: os.Getenv("MAIL_FROM"),
		},
		Auth: &Auth{
			AccCookieName: "access_token",
			AccSecret:     os.Getenv("JWT_ACCESS_SECRET"),
			AccTTL:        10 * time.Minute,
			RefCookieName: "refresh_token",
			RefTTL:        24 * time.Hour,
			Issuer:        "gonext",
			Audience:      "AuthService",
			EmailCodeTTL:  10 * time.Minute,

			RefreshKey:    "refToken:%v",
			EmailSetupKey: "emailSetup:%v|%v",
			EmailLoginKey: "emailLogin:%v",
			EmailPassKey:  "emailPass:%v",
		},
		DB: &DB{
			PostgresUrl:  os.Getenv("POSTGRES_URL"),
			PostgresUser: os.Getenv("POSTGRES_USER"),
			PostgresPass: os.Getenv("POSTGRES_PASSWORD"),
			PostgresDB:   os.Getenv("POSTGRES_DB"),
			RedisURL:     os.Getenv("REDIS_URL"),
		},
		WS: &WS{
			ReadTimeout:    15 * time.Second,
			PongTimeout:    10 * time.Second,
			WriteTimeout:   5 * time.Second,
			MaxMsgSize:     65536, // 64kb
			RegisterBuffer: 20,
			RoomBuffer:     20,
			MsgBuffer:      256,
			SendBuffer:     64,
			RecvBuffer:     64,
		},
		Token: &Token{
			RefTTL:         24 * time.Hour,
			EmailedCodeTTL: 10 * time.Minute,
		},
	}

	return cfg, nil
}

func (c *DB) ConnectionStrings() (string, string) {
	pString := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable",
		c.PostgresUser, c.PostgresPass, c.PostgresUrl, c.PostgresDB)
	rString := fmt.Sprintf("redis://%s", c.RedisURL)
	return pString, rString
}
