# flipbit-nginx
## Overview
The **flipbit-nginx** program is a companion program to **flipbit-core**, designed to create nginx stream configuration files on a load balancer host.

## Definitions

### Load Balancer Host
A load balancer host is a machine that is put into service to handle service traffic from external (outside kubernetes) users/clients.  This machine should have multiple IP addresses bound to it and the ability to hand them out as needed to kubernetes services.

The host 

### 

## Example Setup
The ideal setup is to run _flipbit-nginx_ on a host that is considered a load balancer host