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
	"time"

	"github.com/gorilla/mux"
	kafka "github.com/segmentio/kafka-go"
)

type readertopic struct {
	canal chan kafka.Message
	reader *kafka.Reader
}

var topics map[string]readertopic
var topicsmux sync.Mutex
var kafkaURL string

func consumerHandler() func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		topico := vars["topic"]
		topicsmux.Lock()
		_, ok := topics[topico]
		if !ok {
			topics[topico] = readertopic{canal: make(chan kafka.Message), reader: getKafkaReader(kafkaURL, topico)}
			topicsmux.Unlock()
			err := createTopic(topico)
			if err != nil {
				http.Error(wrt, "Error creando tópico", http.StatusInternalServerError)
				return
			}
			go func() {
				for {
					m, err := topics[topico].reader.FetchMessage(context.Background())
					if err != nil {
						// Ignorar errores
					} else {
						topics[topico].canal <- m
					}
				}
			}()
		} else {
			topicsmux.Unlock()
		}
		select {
		case m := <-topics[topico].canal:
			wrt.Write([]byte(fmt.Sprintf("offset: %v key: %s value: %s", m.Offset, string(m.Key), string(m.Value))))
			topics[topico].reader.CommitMessages(context.Background(),m)
		case <-time.After(time.Second * 10):
			http.Error(wrt, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	})
}

func getKafkaReader(kafkaURL, topic string) *kafka.Reader {
	brokers := strings.Split(kafkaURL, ",")
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "consumer1",
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
	topics = make(map[string]readertopic)
	r := mux.NewRouter()
	r.HandleFunc("/{topic}", consumerHandler()).Methods("GET")
	log.Fatal(http.ListenAndServe(":8072", r))

}
