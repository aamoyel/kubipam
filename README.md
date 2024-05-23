# kubipam

## About the project

Kubipam is a Kubernetes controller that manage network IPs and CIDRs for you.
It works with 2 custom resources:
- IPCidr
- IPClaim

IPCidr allows you to create a specific CIDR in the IPAM (IPv4 and IPv6).
IPClaim is used to allocate/claim specific or non-specific IPs and child CIDRs in the IPAM.

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.

### Setup

1. Install the controller and CRDs:

```sh
kubectl apply -f deploy/bundle.yaml
```

### Usage

### IPCidr

Example definition to create a CIDR in the IPAM:

```yaml
apiVersion: ipam.amoyel.fr/v1alpha1
kind: IPCidr
metadata:
  name: test
spec:
  cidr: 172.16.0.0/16
```

it works as well in IPv6

### Specific

#### IP

Here you can find an example to claim a specific IP address:

```yaml
apiVersion: ipam.amoyel.fr/v1alpha1
kind: IPClaim
metadata:
  name: my-specific-ip
spec:
  type: IP
  ipCidrRef:
    name: mycidr
  specificIPAddress: 172.16.0.1
```

#### CIDR

⚠️ You cannot claim a child cidr when ip addresses are already registered in the parent cidr (ref) !
Here you can find an example to claim a specific child CIDR:

```yaml
apiVersion: ipam.amoyel.fr/v1alpha1
kind: IPClaim
metadata:
  name: my-specific-childcidr
spec:
  type: CIDR
  ipCidrRef:
    name: mycidr
  specificChildCidr: 172.16.1.0/24
```

### Non-specific

#### IP

Here you can find an example to claim the next free IP address:

```yaml
apiVersion: ipam.amoyel.fr/v1alpha1
kind: IPClaim
metadata:
  name: my-ip
spec:
  type: IP
  ipCidrRef:
    name: mycidr
```

#### CIDR

When you claim a child CIDR, you must set the 'length' field but you do not set the 'ipCidrRef' field and the name of a IPCidr resource. The controller will claim a child cidr in a parent cidr that has available addresses.
Here you can find the example:

```yaml
apiVersion: ipam.amoyel.fr/v1alpha1
kind: IPClaim
metadata:
  name: my-cidr
spec:
  type: CIDR
  cidrPrefixLength: 24
```

## Uninstall

Before uninstalling, make sure you do not have any custom resources (IPCidr and IPclaim) applied in your cluster !
To delete the controller and the CRDs from your cluster!

```sh
kubectl delete -f deploy/bundle.yaml
```

## Report bugs

If you think you have found a bug please follow the instructions below.

- Open a new issue.
- Please, write a clear title and describe your bug in the description field.
- Get the logs from the controller or resources status and paste it into your issue.
