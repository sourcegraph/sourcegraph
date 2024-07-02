# Operator Maintenance UI

## Components

This project contains the following components:

### Maintenance UI

A React + Material UI application that communicates with the Operator and gathers data and display status.

Features:

- Installation
- Health & Actions
- Upgrade

### Mock Operator API

In the [mock-api](./mock-api/) folder, a Go Server application that implements the Operator API companion to the Maintenance UI.

#### Mock Operator Debug Bar API

We also implement some test APIs to enable controlling the Mock Operator from the Maitenance UI.

## Running Locally (Developer Mode)

1. Run the go application in the `mock-api` folder:

   ```
   $ cd mock-api
   $ go run ./cmd
   ```

2. Run the Maitenance UI:

   ```
   $ pnpm run dev
   ```

## Building Images

```
$ cd build
$ make
```

It will:

1. Build frontend and backend distributables
2. Build docker images
3. Push images to the container registry
4. Update the Helm chart with the appropriate registry image versions

## Helm Chart

### Preparing the Helm Chart

No action. This step is automated by the image build step.

### Packaging the Helm Chart

TBD

### Installing the Helm Chart

1. Have a Kubernetes cluster configured and available at the command line
2. Test you can access the cluster by running: `kubectl get pods`
3. Install the Helm chart:

   ```
   $ helm install operator ./helm
   ```

   Installer will create the `sourcegraph` namespace

4. Execute the commands output by the installer to get the address of
   the maintenance UI

### Launching the Maintenance UI

Once the data provided by the install step is available,
IP address + maintenance password, open the maintenance UI in your
browser and follow along the wizard.

### Run debug console

Maintenance UI has a debug console that can be used to control flows in the maintenance UI,
to enable set `debugbar: true` in your browser local storage.
