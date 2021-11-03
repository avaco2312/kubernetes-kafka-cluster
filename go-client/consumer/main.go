package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	kafka "github.com/segmentio/kafka-go"
)

var topics map[string]chan string
var topicsmux sync.Mutex
var kafkaURL string

func consumerHandler() func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		topico := vars["topic"]
		topicsmux.Lock()
		_, ok := topics[topico]
		if !ok {
			topics[topico] = make(chan string, 5)
			topicsmux.Unlock()
			err := createTopic(topico)
			if err != nil {
				log.Println("Error creando topico")
			}
			go func() {
				kafkaReader := getKafkaReader(kafkaURL, topico)
				defer kafkaReader.Close()
				for {
					m, err := kafkaReader.ReadMessage(context.Background())
					if err != nil {
						topics[topico] <- "Error leyendo mensaje "+err.Error()+" "+topico
					} else {
						topics[topico] <- fmt.Sprintf("offset: %v key: %s value: %s", m.Offset, string(m.Key), string(m.Value))
					}

				}
			}()
		} else {
			topicsmux.Unlock()
		}
		if len(topics[topico]) == 0 {
			http.Error(wrt, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		wrt.Write([]byte(<-topics[topico]))
	})
}

func getKafkaReader(kafkaURL, topic string) *kafka.Reader {
	brokers := strings.Split(kafkaURL, ",")
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
	})
}

func createTopic(topic string) error {
	conn, err := kafka.Dial("tcp", kafkaURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer controllerConn.Close()
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 3,
		},
	}
	err = controllerConn.CreateTopics(topicConfigs...)
	return err
}

func main() {
	kafkaURL = os.Getenv("KAFKAURL")
	topics = make(map[string]chan string)
	r := mux.NewRouter()
	r.HandleFunc("/{topic}", consumerHandler()).Methods("GET")
	log.Fatal(http.ListenAndServe(":8072", r))

}
