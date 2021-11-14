# The most over-engineered ToDo app ever

## First version
Query endpoints (GET) will return the requested data directly.
Command endpoints (POST, PUT, PATCH, DELETE) will return a correlation ID to be used by the client to track the result of their requests. This implies that after the client submits a request to the BE it should poll it using the correlation ID to check if the command was successful or not.

The BE services should communicate asynchronously using a queue (rabbitMQ) to pass messages btw them.
The main entry point to the app should be a WebAPI and we should have a reverse proxy in front of it to route the requests to the right place. (HAProxy or Nginx).

Containers everywhere.
MongoDB.
Go.

## Second version
We should create a websockets endpoint that will enable server and client to communicate freely.
We should make GET commands to submit requests to the queue to comply with the architectural design.

docker:
- proxy
- db
- queue system
- one testing endpoint (get)

## Running

Main stack:
docker compose up

Restart service:
docker compose up -d --force-recreate --no-deps --build patch-todo

ELK:
 docker compose -f .\docker-compose.elk.yml up -d
 Elastic search may fail to start due to memory constraints. To fix that you can run `sudo sysctl -w vm.max_map_count=262144` on your linux host (or wsl backe bash)

 ### Ports

 80 - proxy/api gateway
 15672 - rabbitmq management portal
 5601 - kibana