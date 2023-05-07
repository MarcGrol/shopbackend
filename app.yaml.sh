#!/bin/bash

cat << Done
runtime: go120
service: default

inbound_services:
- warmup

env_variables:
  LOCATION_ID: "europe-west1"
  QUEUE_NAME: "default"
  ADYEN_ENVIRONMENT: "test"
  ADYEN_MERCHANT_ACCOUNT: "MarcGrolConsultancyECOM"
  ADYEN_API_KEY: ${ADYEN_API_KEY_VAR}
  ADYEN_CLIENT_KEY: ${ADYEN_CLIENT_KEY_VAR}
  ADYEN_OAUTH_CLIENT_ID: "OACL4224X223225R5HPHMJ5DG72QWV"
  ADYEN_OAUTH_CLIENT_SECRET: ${ADYEN_OAUTH_CLIENT_SECRET_VAR}
  ADYEN_OAUTH_AUTH_HOSTNAME: "https://ca-test.adyen.com"
  ADYEN_OAUTH_TOKEN_HOSTNAME: "https://oauth-test.adyen.com"
  STRIPE_ENVIRONMENT: "test"
  STRIPE_OAUTH_CLIENT_ID: "ca_NpMvfkAfZzhiUMnVM5yk3swDpfxD287H"
  STRIPE_OAUTH_CLIENT_SECRET: ${STRIPE_OAUTH_CLIENT_SECRET_VAR}

handlers:
  - url: /.*
    secure: always
    script: auto

Done
