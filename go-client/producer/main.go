package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"github.com/gorilla/mux"
	kafka "github.com/segmentio/kafka-go"
)

var kafkaURL string

func producerHandler(kafkaWriter *kafka.Writer) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
 		topic := vars["topic"]
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
		}
		msg := kafka.Message{
			Topic: topic,
			Key:   []byte(fmt.Sprintf("address-%s", req.RemoteAddr)),
			Value: body,
		}
		err = kafkaWriter.WriteMessages(req.Context(), msg)
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func topicHandler() func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(wrt http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
 		topic := vars["topic"]
		conn, err := kafka.Dial("tcp", kafkaURL)
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		controller, err := conn.Controller()
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
		}
		var controllerConn *kafka.Conn
		controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
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
		if err != nil {
			http.Error(wrt, http.StatusText(http.StatusInternalServerError)+" "+err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func getKafkaWriter(kafkaURL string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(kafkaURL),
		Balancer: &kafka.LeastBytes{},
	}
}

func main() {
	kafkaURL = os.Getenv("KAFKAURL")
	kafkaWriter := getKafkaWriter(kafkaURL)
	defer kafkaWriter.Close()
	r := mux.NewRouter()
	r.HandleFunc("/producer/{topic}", producerHandler(kafkaWriter)).Methods("POST")
	r.HandleFunc("/topic/{topic}", topicHandler()).Methods("POST")
	log.Fatal(http.ListenAndServe(":8070", r))
}
