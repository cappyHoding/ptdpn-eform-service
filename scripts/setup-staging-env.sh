#!/bin/bash

IP="192.168.88.58"
ENV_FILE=".env"

echo "Generating .env for staging at $IP..."

SESSION_SECRET=$(openssl rand -hex 16)
CVV_SECRET=$(openssl rand -hex 16)

cat > $ENV_FILE << EOF
APP_NAME=bpr-perdana-eform
APP_ENV=staging
APP_PORT=8080
APP_BASE_URL=http://$IP/api/v1
FRONTEND_URL=http://$IP
CORS_ALLOWED_ORIGINS=http://$IP,http://$IP:3001

STORAGE_BASE_PATH=/var/app/storage
STORAGE_MAX_KTP_SIZE_MB=5
STORAGE_MAX_COLLATERAL_SIZE_MB=10
STORAGE_MAX_CONTRACT_SIZE_MB=20
STORAGE_LOGO_PATH=/app/assets/bpr-logo.png

DB_HOST=mysql
DB_PORT=3306
DB_NAME=bpr_perdana_eform
DB_USER=eform_user
DB_PASSWORD=eform_password
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=5m

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10

JWT_PRIVATE_KEY_PATH=./keys/private.pem
JWT_PUBLIC_KEY_PATH=./keys/public.pem
JWT_ACCESS_TOKEN_TTL=15m
JWT_REFRESH_TOKEN_TTL=24h

SESSION_SECRET_KEY=$SESSION_SECRET
SESSION_TTL_MINUTES=120

VIDA_OCR_BASE_URL=https://services-sandbox.vida.id
VIDA_OCR_CLIENT_ID=partner-bpr-perdana-snbx-sso
VIDA_OCR_SECRET_KEY=JDm1jOqtcvw5YRjakRvoWw4pG1MK40t0

VIDA_FRAUD_MOCK=true
VIDA_CONTRACT_MOCK=false

# PoA Sign
VIDA_SIGN_BASE_URL=https://services-sandbox.vida.id
VIDA_SIGN_CLIENT_ID=partner-bprperdana-snbx-poa
VIDA_SIGN_SECRET_KEY=2wAYiYR8TF2ktIWtjIg3RSZDObTBF7Pc
VIDA_SIGN_API_KEY=NwTL3NlWytRgSu6W
VIDA_SIGN_CVV=049
VIDA_SIGN_CVV_SECRET_KEY=691f5c989ca5970e42383799913e37fdb15a5d779e2bf47f372928a9ff88f706
VIDA_SIGN_KEY_ID=49dc5c664073bee3aa6031b6c0b634d459d79a48

VIDA_WEB_SDK_CLIENT_ID=partner-bprperdana-snbx-web-sdk
VIDA_WEB_SDK_CLIENT_SECRET=8XhldWZgLhFTLU2T4Br8laJPsHC3aFmE
VIDA_SIGNING_KEY=vqSUmXEB5o/RSOstPRSM5uqhNkA3oyEXMjjFDq1WHNylr/DA8ha6rMK6yQL7qxcJL4y54EgmHjN7zblckd9ckji9vshxzbNDCl8iMuQYqiKypx9l0J1Ck8+Szg7GKeKIFh6AkT3MNV514RXwFAj4GxDMNgqLsNBQTHPa6fo4rWcSaC0RwZPXDetmlS1fMvTrzlAD4qopSSGobgMBSAmDtTCsL4H5A4ni7whMzU6Uu926855VCNkl3zSDIZE=

# eMeterai
VIDA_EMETERAI_BASE_URL=https://sandbox-stamp-gateway.np.vida.id
VIDA_EMETERAI_CLIENT_ID=partner-bprperdana-emeterai-snbx-sso
VIDA_EMETERAI_SECRET_KEY=VsTbprihsJiPHJwJgGQrARNz8lHro4zl
VIDA_EMETERAI_PARTNER_ID=snbx-bprperdana-emeterai

VIDA_HTTP_TIMEOUT=30s

# ── VIDA Direct Sign Platform ──────────────────────────────────────────────────
VIDA_DSIGN_BASE_URL=https://sandbox-sign-api.np.vida.id
VIDA_DSIGN_CLIENT_ID=OLtGuNRWEKxrg77JOCGzOnYqcMPa2avZ
VIDA_DSIGN_SECRET_KEY=DDyRrfvwuO2JRZ7C543LLbHWWT1qEVXfa9TIKwluqhiSHXoGQFHjQ2fdFJMofz5L
VIDA_DSIGN_CREATOR_EMAIL=it_support@bprperdana.com

SMTP_HOST=sandbox.smtp.mailtrap.io
SMTP_PORT=587
SMTP_USERNAME=0ff3f143b0062e
SMTP_PASSWORD=d88c01921c9504
SMTP_FROM_NAME=d88c01921c9504
SMTP_FROM_EMAIL=noreply@bprperdana.co.id

WA_PROVIDER=fonnte
WA_API_URL=https://api.fonnte.com
WA_API_TOKEN=placeholder
WA_FROM_NUMBER=placeholder

RATE_LIMIT_REQUESTS=60
RATE_LIMIT_WINDOW=1m

LOG_LEVEL=debug
LOG_FORMAT=console
LOG_OUTPUT=stdout

VIDA_WEBHOOK_SECRET=bpr-perdana-webhook-secret-2026

# ── IOH SMS Gateway ────────────────────────────────────────────────────────────
IOH_SMS_URL=https://smsapi.three.co.id:25000/sendsms
IOH_SMS_USERNAME=CREG70108
IOH_SMS_PASSWORD=1a165fdb0fcd81b
IOH_SMS_SENDER_ID=BPR PERDANA

# ── Fraud Poller ───────────────────────────────────────────────────────────────
FRAUD_POLL_INTERVAL=30m
EOF

echo "File .env created successfully!"

echo "Checking JWT keys..."
mkdir -p keys
if [ ! -f keys/private.pem ]; then
  echo "Generating JWT keys..."
  openssl genrsa -out keys/private.pem 2048
  openssl rsa -in keys/private.pem -pubout -out keys/public.pem
fi

echo "Done! You can now restart your backend container."
