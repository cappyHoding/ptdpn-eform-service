module github.com/cappyHoding/ptdpn-eform-service

go 1.25.0

// Configuration — reads .env files AND environment variables seamlessly
// Critical for dev (uses .env) vs production (uses real env vars)
require (
	// CORS middleware for Gin
	github.com/gin-contrib/cors v1.7.2

	// Request ID middleware
	github.com/gin-contrib/requestid v1.0.3
	// HTTP framework — lightweight, production-proven, excellent middleware support
	github.com/gin-gonic/gin v1.10.0

	// Input validation — struct tag based validation with Indonesian locale support
	github.com/go-playground/validator/v10 v10.22.0 // indirect

	// MySQL driver — the "pure Go" driver, no CGO dependency needed
	github.com/go-sql-driver/mysql v1.8.1

	// JWT — for internal staff authentication (RS256 asymmetric signing)
	github.com/golang-jwt/jwt/v5 v5.2.1

	// UUID generation — for all our CHAR(36) primary keys
	github.com/google/uuid v1.6.0

	// Redis client — for customer sessions and rate limiting
	github.com/redis/go-redis/v9 v9.5.3

	// Configuration — reads .env files AND environment variables seamlessly
	// Critical for dev (uses .env) vs production (uses real env vars)
	github.com/spf13/viper v1.19.0

	// Structured logging — outputs JSON in production, human-readable in dev
	go.uber.org/zap v1.27.0

	// Password hashing — bcrypt for internal user passwords
	golang.org/x/crypto v0.26.0
	gorm.io/driver/mysql v1.5.7

	// ORM — GORM makes database operations readable without sacrificing control
	// We use it for models + migrations but write raw SQL for complex queries
	gorm.io/gorm v1.25.10
)

require github.com/jung-kurt/gofpdf v1.16.2

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/bytedance/sonic v1.12.2 // indirect
	github.com/bytedance/sonic/loader v0.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.5 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/arch v0.9.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
