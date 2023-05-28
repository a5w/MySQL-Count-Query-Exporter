
# MySQL Count Query Exporter for Prometheus

This is a Prometheus exporter written in Go that executes count queries on a MySQL database and exports the results as Prometheus metrics.

## Usage

To use the MySQL Count Query Exporter:

1.  Clone the repository.
2.  Adjust the YAML configuration file to specify the database connection details and the queries to run.
3.  Build the application with Go.
4.  Run the resulting binary.

### Configuration

The configuration file is a YAML file that contains the details for connecting to your MySQL database and the queries to run.

Example configuration:


```
exporter_port: 2112
db_user: myuser
db_password: mypassword
db_host: myhost
db_port: 3306
exporter_port: 8080
queries:
  - database: mydatabase
    query: SELECT COUNT(*) FROM mytable
    name: my_query
    interval: 60
```

### Building

To build the MySQL Count Query Exporter, run the following command in the root of the repository:

bashCopy code

`go build` 

This will create a binary named `mysql_count_query_exporter`.

### Running

To run the MySQL Count Query Exporter, execute the resulting binary and pass the path to your configuration file with the `-config` flag:


`./mysql_count_query_exporter -config path/to/your/config.yaml` 

The exporter will start running and begin executing the specified queries at the specified intervals. The results will be available as Prometheus metrics at `http://localhost:8080/metrics` (or whatever port you specified in your configuration file).

## Warning

This software is provided "as is", without warranty of any kind, express or implied. Use it at your own risk. Always make sure to test thoroughly in non-production environments before deploying to production. Be aware that executing too many queries too often could impact the performance of your MySQL server.

## Contributing

Contributions are welcome! Please submit a pull request or open an issue if you have a feature request or bug report.

## License

This software is licensed under the MIT license. For more details, see the `LICENSE` file in the repository.