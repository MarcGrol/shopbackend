# shopbackend

Little example app that demonstrates how Adyen drop-in checkout works

## Deploy on Google Appengine

    gcloud auth login 
    gcloud config set project marcsexperiment
    gcloud tasks queues create default --max-attempts=10 --max-concurrent-dispatches=5
    
    cp app.yaml.template app.yaml
    # adjust app.yaml
    gcloud app deploy app.yaml --version v1 --quiet

## Test the userinterface

https://app.marcgrolconsultancy.nl/


## Overview of architecture

![alt text](./adyen_shop_architecture.png)


