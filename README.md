# telegraf-processor-gcp-imds

Telegraf processor which can tag metrics with data from the GCP IMDS service

### Installation

* Clone the repo

```
git clone git@github.com:tuscanylabs/telegraf-processor-gcp-imds.git
```
* Build the "gcp-imds" binary

```
$ go build -o gcp-imds cmd/main.go
```
* You should be able to call this from telegraf now using execd
```
[[processors.gcp_imds]]
  command = ["/path/to/gcp-imds", "-poll_interval 1m"]
  signal = "none"
```
This self-contained plugin is based on the documentations of [Execd Go Shim](https://github.com/influxdata/telegraf/blob/effe112473a6bd8991ef8c12e293353c92f1d538/plugins/common/shim/README.md)

### Unit Testing

The plugin includes unit tests which can be run with the following

```
go test ./...
```

### Functional Testing

Functional tests can be done with the following process

First, create a new GCP VM instance

Next, ensure that the binary is built for the architecture of the VM you created. For example, on an x86-64 arch,
you can specifically build the binary like so.

```
GOOS=linux GOARCH=amd64 go build -o gcp-imds cmd/main.go
```

Next, copy the binary to the instance. You can do this via SCP or any other method you have available.

Next, create a sample config in the VM. You can use the config provided [in this repository][1].

```
[[processors.zone_imds]]
	imds_tags = ["zone"]
```

Next, start the binary with a sample config

```
./gcp-imds --config telegraf.conf
2023/03/03 17:12:10 D! Initializing GCP IMDS Processor
```

Now, paste a line of input to the console using the [InfluxDB line protocol format][2].

```
weather,foo=bar temperature=82 1465839830100400200
```

You should expect to see the output changed to include a `zone` tag. For example,

```
weather,foo=bar,zone=projects/101725147221/zones/us-west1-a temperature=82 1465839830100400200
```

[1]: ./plugins/processors/gcp/imds/sample.conf
[2]: https://docs.influxdata.com/influxdb/v1.8/write_protocols/line_protocol_tutorial/