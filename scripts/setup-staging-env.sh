#!/bin/bash

if [ -z "$1" ]; then
  echo "Usage: ./scripts/setup-staging-env.sh <STAGING_IP>"
  exit 1
fi

IP=$1
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

VIDA_BASE_URL=https://services-sandbox.vida.id
VIDA_CLIENT_ID=mock_client_id
VIDA_CLIENT_SECRET=mock_client_secret
VIDA_DSIGN_BASE_URL=https://sandbox-sign-api.np.vida.id
VIDA_DSIGN_CLIENT_ID=mock_dsign_id
VIDA_DSIGN_CLIENT_SECRET=mock_dsign_secret
VIDA_FRAUD_MOCK=true
VIDA_CONTRACT_MOCK=true

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=mock@example.com
SMTP_PASSWORD=mockpassword
SMTP_FROM_NAME="BPR Perdana"
SMTP_FROM_EMAIL=no-reply@bprperdana.com

IOH_SMS_URL=https://smsapi.three.co.id:25000/sendsms
IOH_SMS_USER=mock
IOH_SMS_PWD=mock

VIDA_SIGN_CVV_SECRET_KEY=$CVV_SECRET
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
