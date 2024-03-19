# How to run postgres commands

This document will take you through how to run postgres queries in the main database for each deployment type.

## Prerequisites

* This document assumes that you are the site admin and have access to your Sourcegraph instance

## Kubernetes

1. Exec into the database to run Postgres queries using `kubectl exec -ti PGSQL-Container-Name -- psql -U sg`

## Docker Compose

1. Exec into the database to run Postgres queries using `docker container exec -it pgsql psql -U sg` 

## Single-container

1. Find the Sourcegraph-Container-ID in which your Sourcegraph instance is running using the following command: `docker ps -a`
1. Run the following command to exec into the container: `docker exec -it Sourcegraph-Container-ID bash`
1. Start postgres to run postgres queries: `psql -U postgres`
1. There is no Password for the database by default, so you can just hit enter when it asks for your password
  
  
