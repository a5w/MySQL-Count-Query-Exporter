package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

// Struct for Queries in yaml file
type Query struct {
	Name     string        `yaml:"name"`
	Databse  string        `yaml:"database"`
	Query    string        `yaml:"query"`
	Interval time.Duration `yaml:"interval"`
}

// Struct for yaml config file
type Config struct {
	Exporter_Port int    `yaml:"exporter_port"`
	DB_Host       string `yaml:"db_host"`
	DB_Port       int    `yaml:"db_port"`
	DB_User       string `yaml:"db_user"`
	DB_Password   string `yaml:"db_password"`
	Queries       []Query
}

// Defining prometheus metric type
var (
	queryMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mysql_query_exporter",
		Help: "The number of rows returned by specified MySQL count queries, labeled by query name and SQL statement.",
	},
		[]string{"name", "query"},
	)
)

func init() {
	prometheus.MustRegister(queryMetric)
}

func readConfig(filename string) (Config, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}

	var config Config

	err = yaml.Unmarshal(bytes, &config)

	if err != nil {
		return Config{}, err
	}

	return config, nil
}

// checkQuery connects to the database, runs a query, and sends the results to Prometheus.
// It uses the provided context to support cancellation.

func checkQuery(user string, password string, host string, port int, database string, query string, name string, interval time.Duration) {
	// Log that the function is attempting to connect to the database
	log.Printf("[%s] Attemping connection", database)

	// Open a connection to the MySQL database
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, host, port, database))

	// If there was an error opening the connection, log it
	if err != nil {
		log.Printf("[%s] Error connecting to database@%s: %v", database, host, err)
	}

	// Log that the connection was established successfully
	log.Printf("[%s] Connection established", database)

	// Ensure the database connection is closed when the function returns
	defer db.Close()

	// Declare a variable to store the result count
	var count int

	// Log that the function is running the provided query
	log.Printf("[%s] Running Query %s", database, query)

	// Run the query and store the result in the count variable
	err = db.QueryRow(query).Scan(&count)

	// If there was an error running the query, log it
	if err != nil {
		log.Printf("[%s] Error executing query %s: %v", database, query, err)
	}

	// Log that the query completed successfully
	log.Printf("[%s] Query complete", database)

	// Log the query result
	log.Printf("[%s] Count: %d", database, count)

	// Send the query result to Prometheus
	queryMetric.WithLabelValues(name, query).Set(float64(count))

	time.Sleep(interval * time.Second)
}

func main() {

	// Define a command line flag for the configuration file path
	configPath := flag.String("config", "query_config.yaml", "path to the YAML configuration file")

	// Parse the flags.
	flag.Parse()

	// Reading config yaml file
	config, err := readConfig(*configPath)

	// If there was an error reading the configuration, log it and exit
	if err != nil {
		log.Fatalf("Error reading hosts yaml file: %v", err)
	}

	// For each query configuration, start a goroutine that periodically runs the query
	for _, conf := range config.Queries {
		go func(conf Query) {
			for {
				checkQuery(config.DB_User, config.DB_Password, config.DB_Host, config.DB_Port, conf.Databse, conf.Query, conf.Name, conf.Interval)
			}
		}(conf)
	}

	// Remove the goroutine that runs the server
	// Instead, create the server and call ListenAndServe directly in the main function
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Exporter_Port),
		Handler: promhttp.Handler(),
	}
	log.Printf("Starting Server on port %d ", config.Exporter_Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// Unexpected error
		log.Fatalf("ListenAndServe(): %v", err)
	}

}
