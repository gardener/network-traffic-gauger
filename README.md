# Network Traffic Gauger

[![reuse compliant](https://reuse.software/badge/reuse-compliant.svg)](https://reuse.software/)

The Network Traffic Gauger determines the network traffic and aggregates the amount of sent/received
data to the corresponding source/destination IP pair. It allows attribution of network traffic to
pods in a kubernetes cluster. The result can be used to determine high bandwidth communication pairs,
but it is also possible given other metadata to determine if communication crosses certain boundaries,
e.g. availability zones or intra/internet boundaries.

## Architecture

The Network Traffic Gauger monitors traffic indirectly using the [netfilter](https://www.netfilter.org)
connection tracking table in the linux kernel. It connects to netfilter using two local sockets:
1. Netfilter emits events for closed connections, which can be used to identify the amount of traffic, which was transferred over the connection.
2. The whole connection tracking table is dumped regularly to be able to gradually increase the metrics over time and not only when the connection is closed.

A prerequisite for this to work is that netfilter accounts the transferred volume per connection.
This can be enabled by running `net-gauger setup`.

The data about transferred volume per source IP is persisted so that there is no loss of data in case
the process is restarted. Per default, data is stored in `/var/log/net-gauger`.

The metrics, i.e. the volume of data being transferred between source and destination IPs, can be queried
in the default [prometheus](https://prometheus.io/) format. Per default, metrics are available on port
`localhost:16160`.

## Deployment

The Network Traffic Gauger can be easily used locally.

*Remark:*

Please note that the tool requires netfilter to be running and assumes linux as operating system.

1. Build the command line tool

   ```
   make build
   ```

2. Run the initialization to enable netfilter accounting

   ```
   net-gauger setup
   ```

3. Start the network traffic measurement process

   ```
   net-gauger run-agent
   ```

4. Optional: Collect the network traffic measurement metrics

   ```
   curl http://localhost:16160/metrics
   ```

### Deployment in a Gardener landscape

TBD
