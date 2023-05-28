package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

func checkQuery(ctx context.Context, user string, password string, host string, port int, database string, query string, name string, interval time.Duration) {
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

	// Wait for either the context to be cancelled or for the interval to pass
	select {
	case <-time.After(interval * time.Second):
		// Sleep duration elapsed
	case <-ctx.Done():
		// Context cancelled
		return
	}
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

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Capture SIGINT and SIGTERM signals to stop goroutines cleanly

	// Create a channel to receive OS signals
	signalCh := make(chan os.Signal, 1)

	// Configure the program to send an interrupt signal (SIGINT) or termination signal (SIGTERM) to the signalCh channel
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine that waits for a signal and then cancels the context
	go func() {
		sig := <-signalCh
		fmt.Printf("Received signal: %s. Exiting...\n", sig)
		cancel() // This will cancel the context
		fmt.Println("Cancel function called.")
	}()

	// For each query configuration, start a goroutine that periodically runs the query
	for _, conf := range config.Queries {
		go func(conf Query) {
			ticker := time.NewTicker(conf.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					fmt.Println("Received done signal. Exiting goroutine...")
					// Clean up and stop go routine
					return
				case <-ticker.C:
					checkQuery(ctx, config.DB_User, config.DB_Password, config.DB_Host, config.DB_Port, conf.Databse, conf.Query, conf.Name, conf.Interval)
				}
			}
		}(conf)
	}

	// Create an instance of the http.Server struct. This allows for more control
	// over the HTTP server configuration and lifecycle than using http.ListenAndServe directly.
	srv := &http.Server{
		// Addr field is the TCP address for the server to listen on. Here it's set to the port specified in the config.
		Addr: fmt.Sprintf(":%d", config.Exporter_Port),
		// Handler field is the http.Handler to invoke. promhttp.Handler() returns an HTTP handler
		// that exposes the default Prometheus registry as an HTTP endpoint.
		Handler: promhttp.Handler(),
	}

	// Start the server in a separate goroutine so that it doesn't block the main function.
	// This allows the main function to continue and listen for the context cancellation.
	go func() {
		// Log the start of the server.
		log.Printf("Starting Server on port %d ", config.Exporter_Port)

		// Call ListenAndServe on the server. This will block until the server is stopped.
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// If the server is closed normally, ListenAndServe returns http.ErrServerClosed.
			// If it returns any other error, log this as a fatal error.
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Block and wait for the context to be cancelled. This could be due to receiving a shutdown signal
	// (like SIGINT or SIGTERM) or due to a call to cancel function somewhere else in your program.
	<-ctx.Done()

	// Once the context is cancelled, log a shutdown message and attempt to gracefully shutdown the server.
	// This involves finishing all current requests and then closing the server.
	log.Println("Shutting down the server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		// If the server cannot be shutdown cleanly, log the error.
		log.Printf("Could not shutdown server: %v", err)
	}

}
