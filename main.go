package main

import (
	"math/rand"
	"net/http"
	"syscall"

	"time"

	"flag"

	"fmt"

	"sync"

	"os"

	"bitbucket.org/tapgerine/traffic_rotator/rotator"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/config"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/data"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/redis_handler"
	"bitbucket.org/tapgerine/traffic_rotator/rotator/request_context"
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"github.com/go-redis/redis"
	"github.com/oschwald/maxminddb-golang"
)

func init() {
	rand.Seed(time.Now().UnixNano())

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stdout)
	//hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	//if err != nil {
	//	log.Error("Unable to connect to local syslog daemon")
	//} else {
	//	log.AddHook(hook)
	//}
}

// Main Program
func main() {
	var err error

	var (
		kafkaBrokers             = flag.String("kafka_brokers", "localhost:9092", "Kafka brokers")
		redisHost                = flag.String("redis_host", "localhost", "Redis host")
		redisPwd                 = flag.String("redis_pwd", "", "Redis password")
		encryptionKey            = flag.String("encryption_key", "", "Encryption key")
		logFilePath              = flag.String("log_file", "/tmp/traffic_rotator_log", "Log file for critical errors")
		isCriticalLoggingEnabled = flag.String("is_log_enabled", "false", "Is critical logging enabled")
		geoDBFile                = flag.String("geo_file", "geo_db/GeoIP2-Country.mmdb", "Geo db file location")
		rotatorDomain            = flag.String("rotator_domain", "pmp.tapgerine.com", "Rotator domain")
		statsDomain              = flag.String("stats_domain", "pmp-stats.tapgerine.com", "Stats domain")
	)
	flag.Parse()

	//agent := stackimpact.Start(stackimpact.Options{
	//	AgentKey: "4780ddeda961dd38d284b3fa10a3ddaf7c5bc1ad",
	//	AppName:  "Rotator",
	//})

	if *encryptionKey == "" {
		log.Warn("No encryption key")
		return
	}

	if *isCriticalLoggingEnabled == "true" {
		crashLogFileName := fmt.Sprintf(`%s_%d`, *logFilePath, time.Now().UTC().Unix())
		crashLogFile, _ := os.OpenFile(crashLogFileName, os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
		syscall.Dup2(int(crashLogFile.Fd()), 1)
		syscall.Dup2(int(crashLogFile.Fd()), 2)
	}

	rotator.EncryptionKey = []byte(*encryptionKey)
	data.ServingData = &data.ParsedServingData{
		DataWriteLock: sync.RWMutex{},
	}

	redis_handler.RedisConnection = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", *redisHost),
		Password: *redisPwd,
		DB:       0,
	})

	brokers := []string{*kafkaBrokers}
	//setup relevant config info
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Partitioner = sarama.NewRandomPartitioner
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Flush.Frequency = 10000 * time.Millisecond
	rotator.KafkaProducer, err = sarama.NewAsyncProducer(brokers, saramaConfig)

	if err != nil {
		log.WithError(err).Warn()
		panic(err)
	}

	defer func() {
		if err := rotator.KafkaProducer.Close(); err != nil {
			log.WithError(err).Warn()
			panic(err)
		}
	}()

	request_context.GeoDatabase, err = maxminddb.Open(*geoDBFile)
	if err != nil {
		log.WithError(err).Warn()
		panic(err)
	}
	defer request_context.GeoDatabase.Close()

	config.RotatorDomain = *rotatorDomain
	config.StatsDomain = *statsDomain

	//runtime.GOMAXPROCS(runtime.NumCPU())
	log.Info("Application working on port 8081")
	//http.HandleFunc(agent.MeasureHandlerFunc("/rotator", rotator.AdRotationHandler))
	//http.HandleFunc(agent.MeasureHandlerFunc("/rotator/target", rotator.AdRotationTargetingHandler))
	//http.HandleFunc(agent.MeasureHandlerFunc("/rotator/target/v2", rotator.AdRotationTargetingHandlerV2))
	//http.HandleFunc(agent.MeasureHandlerFunc("/rotator/target/bidder_init", rotator.AdRotationOpenRTBInitHandler))
	//http.HandleFunc(agent.MeasureHandlerFunc("/rotator/target/bidder_processor", rotator.AdRotationOpenRTBProcessorHandler))
	//http.HandleFunc(agent.MeasureHandlerFunc("/single_page/get_data/", rotator.SinglePageUserData))
	http.HandleFunc("/rotator", rotator.AdRotationHandler)
	http.HandleFunc("/rotator/target", rotator.AdRotationTargetingHandler)
	http.HandleFunc("/rotator/target/v2", rotator.AdRotationTargetingHandlerV2)
	http.HandleFunc("/rotator/target/bidder_init", rotator.AdRotationOpenRTBInitHandler)
	http.HandleFunc("/rotator/target/bidder_processor", rotator.AdRotationOpenRTBProcessorHandler)
	http.HandleFunc("/single_page/get_data/", rotator.SinglePageUserData)
	log.Fatal(http.ListenAndServe(":8081", nil))
}
