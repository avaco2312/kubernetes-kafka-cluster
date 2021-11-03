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

type topic struct {
	buffer []string
	sync.Mutex
}

var topics map[string]*topic
var topicsmux sync.Mutex
var kafkaURL string

func consumerHandler() func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		topico := vars["topic"]
		topicsmux.Lock()
		ctopico, ok := topics[topico]
		topicsmux.Unlock()
		if !ok {
			ctopico = &topic{buffer: []string{}}
			err := createTopic(topico)
			if err != nil {
				log.Println("Error creando topico")
			}
			go func() {
				kafkaReader := getKafkaReader(kafkaURL, topico)
				defer kafkaReader.Close()
				for {
					m, err := kafkaReader.ReadMessage(context.Background())
					ctopico.Lock()
					if err != nil {
						ctopico.buffer = append(ctopico.buffer, "Error leyendo mensaje "+err.Error()+" "+topico)
					} else {
						ctopico.buffer = append(ctopico.buffer, fmt.Sprintf("offset: %v key: %s value: %s", m.Offset, string(m.Key), string(m.Value)))
					}
					topicsmux.Lock()
					topics[topico] = ctopico
					topicsmux.Unlock()
					ctopico.Unlock()
				}
			}()
		}
		ctopico.Lock()
		defer ctopico.Unlock()
		if len(ctopico.buffer) == 0 {
			http.Error(wrt, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		wrt.Write([]byte(ctopico.buffer[0]))
		ctopico.buffer = ctopico.buffer[1:]
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
	topics = make(map[string]*topic)
	r := mux.NewRouter()
	r.HandleFunc("/{topic}", consumerHandler()).Methods("GET")
	log.Fatal(http.ListenAndServe(":8072", r))

}
