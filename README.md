# Example of doing payments with the Adyen, Stripe and Mollie platform

Example app that demonstrates how Adyen, Stripe and Mollie checkout and OAuth work an all 3 platforms

## Play around with the app

[https://www.marcgrolconsultancy.nl](https://www.marcgrolconsultancy.nl/)

## Architecture

![Overview if architecture](https://github.com/MarcGrol/shopbackend/blob/main/docs/integration_experiment_architecture.png)


## Manual deployment on Google Appengine

    # Login in to gcloud to start using the cli
    gcloud auth login 
    gcloud config set project <your-project-name>   
    
    # Prepare a task-queue
    gcloud tasks queues create default --max-attempts=10 --max-concurrent-dispatches=5
    
    # Create your own app.yaml
    cp app_example.yaml app.yaml # and set env-vars to the right values
    
    # Perform the actual deployment
    gcloud app deploy app.yaml index.yaml cron.yaml --quiet


