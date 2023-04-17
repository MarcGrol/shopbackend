#!/bin/bash

cat << Done
runtime: go120
service: default

env_variables:
  LOCATION_ID: "europe-west1"
  QUEUE_NAME: "default"
  ADYEN_ENVIRONMENT: "test"
  ADYEN_MERCHANT_ACCOUNT: "MarcGrolConsultancyECOM"
  ADYEN_API_KEY: ${ADYEN_API_KEY_VAR}
  ADYEN_CLIENT_KEY: ${ADYEN_CLIENT_KEY_VAR}
  OAUTH_CLIENT_ID: "OACL4224X223225R5HPHMJ5DG72QWV"
  OAUTH_CLIENT_SECRET: ${OAUTH_CLIENT_SECRET_VAR}
  OAUTH_AUTH_HOSTNAME: "https://ca-test.adyen.com"
  OAUTH_TOKEN_HOSTNAME: "https://oauth-test.adyen.com"

handlers:
  - url: /.*
    secure: always
    script: auto

Done