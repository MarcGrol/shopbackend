# Example of drop-in interacting with the Adyen payment platform

Example app that demonstrates how Adyen OAuth and drop-in checkout works

## Play around with the app

[https://www.marcgrolconsultancy.nl](https://www.marcgrolconsultancy.nl/)

## Manual deployment on Google Appengine

    # Login in to gcloud to start using the cli
    gcloud auth login 
    gcloud config set project <your-project-name>   
    
    # Prepare a task-queue
    gcloud tasks queues create default --max-attempts=10 --max-concurrent-dispatches=5
    
    # Create your own app.yaml
    cp app_example.yaml app.yaml # and set env-vars to the right values
    
    # Perform the actual deployment
    gcloud app deploy app.yaml --version version1 --quiet

## Overview of architecture

![alt text](./docs/adyen_shop_architecture.png)
