# Introducing a new service

Before reading this document be sure to first check out our [architecture overview](https://docs.sourcegraph.com/dev/background-information/architecture).

## Terminology

When we say "service" here we are referring to code that runs _within a Docker container_. This may be one or more processes, goroutines, threads, etc. operating in a single Docker container.

## When does introducing a new service make sense?

Sourcegraph is composed of several smaller services (gitserver, searcher, symbols, etc.) and a single monolithic service (the frontend).

When thinking of adding a new service, it is important to think through the following questions carefully:

1. Does the code belong in an existing service?
2. Instead of introducing a new service, could the process/etc reasonably live inside another service (the frontend, etc.) as a background worker, goroutine, etc.?
3. Is the code heavily coupled to an existing service logic? ie. Changes to the other service will likely require changes to this service. If so, have you considered putting it in that service instead?
4. If you make the change within an existing service, instead of a new one, would it substantially increase the complexity of the task at hand?
    - For example, the service you are writing _must_ be written in language X and it is impossible/very difficult to integrate language X into one of our existing Go services.
5. Does it need its own resource constraints and scaling?
   - For example, the service you are creating needs its own CPU / memory resource constraints, or must be able to scale horizontally across machines.

If after asking the above questions to yourself you still believe introducing a new service is the best approach forward, you should [create an RFC](https://handbook.sourcegraph.com/communication/rfcs) proposing it to the rest of the team. In your RFC, be sure to answer the above questions to explain why you believe a separate service is a better choice than integration into an existing service.

### Services have additional overhead for us and users that is easy to forget

Introducing a new service means additional complexity and overhead for us and users. While they may provide a good way to isolate code, services are very much not free. Of course, a monolith is not free or necessarily cheaper either. The pros and cons _in our situation, not based on preference_ must be weighed.

When introducing a new service/container we pay the cost of:

- Introducing and maintaining [its Kubernetes YAML](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/base).
- Adding it to our [docker-compose deployments](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/38).
- Integrating it as a raw process in [the single-container `sourcegraph/server` deployment mode](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/server).
- Documenting clearly [how it scales](https://docs.sourcegraph.com/admin/install/kubernetes/scale) alongside other services for cluster deployments.
- Updating our [architecture diagram](https://docs.sourcegraph.com/dev/background-information/architecture).
- Documenting the service itself in general and how site admins should manage and debug it (these needs to be done regardless of it being a new service, but if it is a new service there are additional aspects to consider.)
- [Updating deploy-sourcegraph-docker](https://github.com/sourcegraph/deploy-sourcegraph-docker) - including testing that it works, documenting which other services it speaks to, and notifying the customers relying on this deployment documentation to deploy this service on their own.
  - Note: We must advise these customers **exactly** which new container has been added, how to deploy it and with what configuration, describe what it does, why we've added it, and assist with that process. Remember, these users are not just running the scripts in our repository -- they are effectively deploying these containers arbitrarily on their own infrastructure with our guidance.
 - Updating the [resource estimator](https://docs.sourcegraph.com/admin/install/resource_estimator) to provide details on resource requirements at different scales.
 - Training new and existing Sourcegraph team members how to interact with and debug the service, as well as customers and our Customer Engineering team (this needs to happen for the feature/change regardless, but as a new service there are some additional aspects.)

Do not introduce a new service/container just for sake of code seperation. Instead, look for alternatives that allow you to achieve the same logical code seperation within the right existing service/container (goroutines, multiple processes in a container, etc. are all valid options.)

Introducing a new service/container can make sense in some circumstances, it is very important to weigh the pros and cons of each based on the circumstance and consider if the value gained is worth it or not.
