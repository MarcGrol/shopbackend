# Example shop that interacts with the Adyen paymnent platform

Little example app that demonstrates how Adyen drop-in checkout works

## Play around with the app

https://marcsexperiment.ew.r.appspot.com/


## Deploy on Google Appengine

    # Login in to gcloud to start using the cli
    gcloud auth login 
    gcloud config set project <your-project-name>   
    
    # Prepare a task-queue
    gcloud tasks queues create default --max-attempts=10 --max-concurrent-dispatches=5
    
    # Create your own app.yaml
    cp app_example.yaml.template app.yaml # set env-vars to the right values
    
    # Perform the actual deployment
    gcloud app deploy app.yaml --version version1 --quiet

## Overview of architecture

![alt text](./docs/adyen_shop_architecture.png)

## OAuth

### Auth-url

    https://ca-test.adyen.com/ca/ca/oauth/connect.shtml?
        client_id=4G63CsWtgfmz3x4aPjHZqgvA8JpU6f9R
        &code_challenge=n-Sg2fMz4TCQdOn6HBdocaISVYzRlNGTWu-a3zxK5cQ
        &code_challenge_method=S256
        &redirect_uri=http%3A%2F%2Flocalhost%3A8082%2Foauth%2Fdone
        &response_type=code
        &scope=psp.onlinepayment%3Awrite+psp.accountsettings%3Awrite+psp.webhook%3Awrite
        &state=3147fa78-1168-4732-901a-185d2295ebc4

### Get-token-request

    POST /v1/token HTTP/1.1
        Accept: application/x-www-form-urlencoded
        Authorization: Basic MTIzOjQ1Ng==
        Content-Type: application/x-www-form-urlencoded

        client_id=123
        &code=mycode
        &code_verifier=exampleHash
        &grant_type=authorization_code
        &redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Foauth%2Fdone


